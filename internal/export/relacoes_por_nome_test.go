package export

import (
	"context"
	"strings"
	"testing"

	"github.com/Felipe-Cavalca/TraceSQL/internal/config"
)

// Casos de exportador de docs/RELACOES.md (1, 5, 14, 21 e 35 a 38).

const rpnFKUsers = `FOREIGN KEY ("user_id") REFERENCES "users" ("id")`

func rpnExporta(t *testing.T, cfg config.Config, ddl []string) string {
	t.Helper()
	db := openSQLiteTestDB(t)
	for _, q := range ddl {
		mustExec(t, db, q)
	}
	cfg.Driver = "sqlite"
	cfg.OutputDriver = "postgres"
	cfg.DSN = "unused"
	cfg.Column = "id"
	dump, err := Run(context.Background(), db, cfg)
	if err != nil {
		t.Fatalf("erro ao exportar: %v", err)
	}
	return dump
}

func rpnNaoContem(t *testing.T, dump string, trechos ...string) {
	t.Helper()
	for _, s := range trechos {
		if strings.Contains(dump, s) {
			t.Fatalf("o dump não deveria conter %q:\n%s", s, dump)
		}
	}
}

func rpnEsquemaUsersOrders() []string {
	return []string{
		"CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT)",
		"CREATE TABLE orders (id INTEGER PRIMARY KEY, user_id INTEGER, total INT)",
		"INSERT INTO users (id, name) VALUES (1, 'Alice'), (2, 'Bob')",
		"INSERT INTO orders (id, user_id, total) VALUES (10, 1, 50), (20, 2, 99)",
	}
}

// Caso 1 e caso 38: flag ligada ponta a ponta.
func TestRelacoesPorNomeExportadorCaso1e38PluralComS(t *testing.T) {
	dump := rpnExporta(t, config.Config{Table: "users", Record: "1", RelationsByName: true}, rpnEsquemaUsersOrders())

	containsAll(t, dump,
		`CREATE TABLE IF NOT EXISTS "orders"`,
		rpnFKUsers,
		`INSERT INTO "orders" ("id", "user_id", "total") VALUES (10, 1, 50);`,
		`INSERT INTO "users" ("id", "name") VALUES (1, 'Alice');`,
	)
	rpnNaoContem(t, dump, "(20, 2, 99)", "'Bob'")
}

// Caso 5: coluna no plural.
func TestRelacoesPorNomeExportadorCaso5ColunaNoPlural(t *testing.T) {
	dump := rpnExporta(t, config.Config{Table: "users", Record: "1", RelationsByName: true}, []string{
		"CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT)",
		"CREATE TABLE orders (id INTEGER PRIMARY KEY, users_id INTEGER)",
		"INSERT INTO users (id, name) VALUES (1, 'Alice')",
		"INSERT INTO orders (id, users_id) VALUES (10, 1)",
	})
	containsAll(t, dump,
		`FOREIGN KEY ("users_id") REFERENCES "users" ("id")`,
		`INSERT INTO "orders" ("id", "users_id") VALUES (10, 1);`,
	)
}

// Caso 14: prefixo na coluna não infere relação.
func TestRelacoesPorNomeExportadorCaso14PrefixoNaColuna(t *testing.T) {
	dump := rpnExporta(t, config.Config{Table: "addresses", Record: "1", RelationsByName: true}, []string{
		"CREATE TABLE addresses (id INTEGER PRIMARY KEY, street TEXT)",
		"CREATE TABLE orders (id INTEGER PRIMARY KEY, billing_address_id INTEGER)",
		"INSERT INTO addresses (id, street) VALUES (1, 'Rua A')",
		"INSERT INTO orders (id, billing_address_id) VALUES (10, 1)",
	})
	containsAll(t, dump, `INSERT INTO "addresses" ("id", "street") VALUES (1, 'Rua A');`)
	rpnNaoContem(t, dump, `"orders"`, "FOREIGN KEY")
}

// Caso 21: tabela inexistente é ignorada, sem erro.
func TestRelacoesPorNomeExportadorCaso21TabelaInexistente(t *testing.T) {
	dump := rpnExporta(t, config.Config{Table: "orders", Record: "10", RelationsByName: true}, []string{
		"CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT)",
		"CREATE TABLE orders (id INTEGER PRIMARY KEY, coupon_id INTEGER)",
		"INSERT INTO users (id, name) VALUES (1, 'Alice')",
		"INSERT INTO orders (id, coupon_id) VALUES (10, 1)",
	})
	containsAll(t, dump, `INSERT INTO "orders" ("id", "coupon_id") VALUES (10, 1);`)
	rpnNaoContem(t, dump, "FOREIGN KEY", `"users"`)
}

// Caso 35: destino ignorado por --ignore-table-suffix.
func TestRelacoesPorNomeExportadorCaso35DestinoIgnorado(t *testing.T) {
	dump := rpnExporta(t, config.Config{Table: "orders", Record: "10", RelationsByName: true, IgnoreTableSuffix: "_log"}, []string{
		"CREATE TABLE user_log (id INTEGER PRIMARY KEY, msg TEXT)",
		"CREATE TABLE orders (id INTEGER PRIMARY KEY, user_log_id INTEGER)",
		"INSERT INTO user_log (id, msg) VALUES (1, 'x')",
		"INSERT INTO orders (id, user_log_id) VALUES (10, 1)",
	})
	containsAll(t, dump, `INSERT INTO "orders" ("id", "user_log_id") VALUES (10, 1);`)
	rpnNaoContem(t, dump, "user_log\"", "'x'", "FOREIGN KEY")
}

// Caso 36: tabela de origem ignorada por --ignore-table-suffix.
func TestRelacoesPorNomeExportadorCaso36OrigemIgnorada(t *testing.T) {
	dump := rpnExporta(t, config.Config{Table: "users", Record: "1", RelationsByName: true, IgnoreTableSuffix: "_log"}, []string{
		"CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT)",
		"CREATE TABLE orders_log (id INTEGER PRIMARY KEY, user_id INTEGER)",
		"INSERT INTO users (id, name) VALUES (1, 'Alice')",
		"INSERT INTO orders_log (id, user_id) VALUES (5, 1)",
	})
	containsAll(t, dump, `INSERT INTO "users" ("id", "name") VALUES (1, 'Alice');`)
	rpnNaoContem(t, dump, "orders_log", "FOREIGN KEY")
}

// Caso 37: flag desligada.
func TestRelacoesPorNomeExportadorCaso37FlagDesligada(t *testing.T) {
	dump := rpnExporta(t, config.Config{Table: "users", Record: "1", RelationsByName: false}, rpnEsquemaUsersOrders())

	containsAll(t, dump, `INSERT INTO "users" ("id", "name") VALUES (1, 'Alice');`)
	rpnNaoContem(t, dump, `"orders"`, "FOREIGN KEY")
}
