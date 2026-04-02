package export

import (
	"context"
	"strings"
	"testing"

	"github.com/Felipe-Cavalca/TraceSQL/internal/config"
)

func TestRunExportSQLiteParaPostgresIncluiSchemaERelacoesTransitivas(t *testing.T) {
	db := openSQLiteTestDB(t)

	mustExec(t, db, "PRAGMA foreign_keys = ON")
	mustExec(t, db, "CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT)")
	mustExec(t, db, "CREATE TABLE orders (id INTEGER PRIMARY KEY, user_id INTEGER, total INT, FOREIGN KEY(user_id) REFERENCES users(id))")
	mustExec(t, db, "CREATE TABLE order_items (id INTEGER PRIMARY KEY, order_id INTEGER, sku TEXT, FOREIGN KEY(order_id) REFERENCES orders(id))")
	mustExec(t, db, "INSERT INTO users (id, name) VALUES (1, 'Alice'), (2, 'Bob')")
	mustExec(t, db, "INSERT INTO orders (id, user_id, total) VALUES (10, 1, 50), (11, 1, 70), (20, 2, 99)")
	mustExec(t, db, "INSERT INTO order_items (id, order_id, sku) VALUES (100, 10, 'A-1'), (101, 11, 'A-2'), (200, 20, 'B-1')")

	cfg := config.Config{
		Driver:       "sqlite",
		OutputDriver: "postgres",
		DSN:          "unused",
		Table:        "orders",
		Column:       "id",
		Record:       "10",
	}

	sqlDump, err := Run(context.Background(), db, cfg)
	if err != nil {
		t.Fatalf("erro ao exportar: %v", err)
	}

	containsAll(t, sqlDump,
		`CREATE TABLE IF NOT EXISTS "users"`,
		`CREATE TABLE IF NOT EXISTS "orders"`,
		`CREATE TABLE IF NOT EXISTS "order_items"`,
		`FOREIGN KEY ("user_id") REFERENCES "users" ("id")`,
		`FOREIGN KEY ("order_id") REFERENCES "orders" ("id")`,
		`INSERT INTO "users" ("id", "name") VALUES (1, 'Alice');`,
		`INSERT INTO "orders" ("id", "user_id", "total") VALUES (10, 1, 50);`,
		`INSERT INTO "orders" ("id", "user_id", "total") VALUES (11, 1, 70);`,
		`INSERT INTO "order_items" ("id", "order_id", "sku") VALUES (100, 10, 'A-1');`,
		`INSERT INTO "order_items" ("id", "order_id", "sku") VALUES (101, 11, 'A-2');`,
	)

	if strings.Contains(sqlDump, `'Bob'`) || strings.Contains(sqlDump, "(20, 2, 99)") || strings.Contains(sqlDump, "'B-1'") {
		t.Fatalf("o dump não deveria incluir registros desconectados do grafo inicial: %s", sqlDump)
	}

	assertInOrder(t, sqlDump,
		`CREATE TABLE IF NOT EXISTS "users"`,
		`CREATE TABLE IF NOT EXISTS "orders"`,
		`CREATE TABLE IF NOT EXISTS "order_items"`,
		`INSERT INTO "users"`,
		`INSERT INTO "orders"`,
		`INSERT INTO "order_items"`,
	)
}

func TestRunExportSQLiteParaMySQLGeraSQLNoDialetoDestino(t *testing.T) {
	db := openSQLiteTestDB(t)

	mustExec(t, db, "PRAGMA foreign_keys = ON")
	mustExec(t, db, "CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT)")
	mustExec(t, db, "CREATE TABLE orders (id INTEGER PRIMARY KEY, user_id INTEGER, total INT, FOREIGN KEY(user_id) REFERENCES users(id))")
	mustExec(t, db, "INSERT INTO users (id, name) VALUES (1, 'Alice')")
	mustExec(t, db, "INSERT INTO orders (id, user_id, total) VALUES (10, 1, 99)")

	cfg := config.Config{
		Driver:       "sqlite",
		OutputDriver: "mysql",
		DSN:          "unused",
		Table:        "users",
		Column:       "id",
		Record:       "1",
	}

	sqlDump, err := Run(context.Background(), db, cfg)
	if err != nil {
		t.Fatalf("erro ao exportar: %v", err)
	}

	containsAll(t, sqlDump,
		"CREATE TABLE IF NOT EXISTS `users`",
		"CREATE TABLE IF NOT EXISTS `orders`",
		"`id` INT AUTO_INCREMENT NOT NULL",
		"FOREIGN KEY (`user_id`) REFERENCES `users` (`id`)",
		"INSERT INTO `users` (`id`, `name`) VALUES (1, 'Alice');",
		"INSERT INTO `orders` (`id`, `user_id`, `total`) VALUES (10, 1, 99);",
	)

	if strings.Contains(sqlDump, `"users"`) {
		t.Fatalf("a saída mysql não deveria usar identificadores no formato postgres: %s", sqlDump)
	}
}

func TestRunExportNewIDsComRelacoesNoDestinoSQLite(t *testing.T) {
	db := openSQLiteTestDB(t)

	mustExec(t, db, "PRAGMA foreign_keys = ON")
	mustExec(t, db, "CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT)")
	mustExec(t, db, "CREATE TABLE orders (id INTEGER PRIMARY KEY, user_id INTEGER, total INT, FOREIGN KEY(user_id) REFERENCES users(id))")
	mustExec(t, db, "CREATE TABLE order_items (id INTEGER PRIMARY KEY, order_id INTEGER, sku TEXT, FOREIGN KEY(order_id) REFERENCES orders(id))")
	mustExec(t, db, "INSERT INTO users (id, name) VALUES (1, 'Alice')")
	mustExec(t, db, "INSERT INTO orders (id, user_id, total) VALUES (10, 1, 99)")
	mustExec(t, db, "INSERT INTO order_items (id, order_id, sku) VALUES (100, 10, 'A-1')")

	cfg := config.Config{
		Driver:       "sqlite",
		OutputDriver: "sqlite",
		DSN:          "unused",
		Table:        "users",
		Column:       "id",
		Record:       "1",
		NewIDs:       true,
	}

	sqlDump, err := Run(context.Background(), db, cfg)
	if err != nil {
		t.Fatalf("erro ao exportar com new_ids: %v", err)
	}

	containsAll(t, sqlDump,
		"CREATE TEMP TABLE `__tracesql_map_users`",
		"CREATE TEMP TABLE `__tracesql_map_orders`",
		"CREATE TEMP TABLE `__tracesql_map_order_items`",
		"INSERT INTO `users` (`name`) VALUES ('Alice');",
		"INSERT INTO `__tracesql_map_users` (`old_id`, `new_id`) VALUES (1, last_insert_rowid());",
		"INSERT INTO `orders` (`user_id`, `total`) VALUES ((SELECT `new_id` FROM `__tracesql_map_users` WHERE `old_id` = 1), 99);",
		"INSERT INTO `__tracesql_map_orders` (`old_id`, `new_id`) VALUES (10, last_insert_rowid());",
		"INSERT INTO `order_items` (`order_id`, `sku`) VALUES ((SELECT `new_id` FROM `__tracesql_map_orders` WHERE `old_id` = 10), 'A-1');",
	)
}

func TestRunExportNewIDsComRelacoesNoDestinoPostgres(t *testing.T) {
	db := openSQLiteTestDB(t)

	mustExec(t, db, "PRAGMA foreign_keys = ON")
	mustExec(t, db, "CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT)")
	mustExec(t, db, "CREATE TABLE orders (id INTEGER PRIMARY KEY, user_id INTEGER, total INT, FOREIGN KEY(user_id) REFERENCES users(id))")
	mustExec(t, db, "INSERT INTO users (id, name) VALUES (1, 'Alice')")
	mustExec(t, db, "INSERT INTO orders (id, user_id, total) VALUES (10, 1, 99)")

	cfg := config.Config{
		Driver:       "sqlite",
		OutputDriver: "postgres",
		DSN:          "unused",
		Table:        "users",
		Column:       "id",
		Record:       "1",
		NewIDs:       true,
	}

	sqlDump, err := Run(context.Background(), db, cfg)
	if err != nil {
		t.Fatalf("erro ao exportar com new_ids para postgres: %v", err)
	}

	containsAll(t, sqlDump,
		`CREATE TEMP TABLE "__tracesql_map_users"`,
		`CREATE TEMP TABLE "__tracesql_map_orders"`,
		`WITH inserted AS (`,
		`INSERT INTO "users" ("name") VALUES ('Alice') RETURNING "id"`,
		`INSERT INTO "__tracesql_map_users" ("old_id", "new_id")`,
		`INSERT INTO "orders" ("user_id", "total") VALUES ((SELECT "new_id" FROM "__tracesql_map_users" WHERE "old_id" = 1), 99) RETURNING "id"`,
	)
}

func TestRunExportNaoInfereRelacoesPorNomePorPadrao(t *testing.T) {
	db := openSQLiteTestDB(t)

	mustExec(t, db, "CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT)")
	mustExec(t, db, "CREATE TABLE orders (id INTEGER PRIMARY KEY, user_id INTEGER, total INT)")
	mustExec(t, db, "INSERT INTO users (id, name) VALUES (1, 'Alice')")
	mustExec(t, db, "INSERT INTO orders (id, user_id, total) VALUES (10, 1, 99)")

	cfg := config.Config{
		Driver:       "sqlite",
		OutputDriver: "sqlite",
		DSN:          "unused",
		Table:        "users",
		Column:       "id",
		Record:       "1",
	}

	sqlDump, err := Run(context.Background(), db, cfg)
	if err != nil {
		t.Fatalf("erro ao exportar sem inferência por nome: %v", err)
	}

	containsAll(t, sqlDump,
		"CREATE TABLE IF NOT EXISTS `users`",
		"INSERT INTO `users` (`id`, `name`) VALUES (1, 'Alice');",
	)

	if strings.Contains(sqlDump, "CREATE TABLE IF NOT EXISTS `orders`") || strings.Contains(sqlDump, "FOREIGN KEY (`user_id`) REFERENCES `users` (`id`)") {
		t.Fatalf("não deveria inferir relações por nome sem a flag: %s", sqlDump)
	}
}

func TestRunExportRelacoesPorNomeSemForeignKey(t *testing.T) {
	db := openSQLiteTestDB(t)

	mustExec(t, db, "CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT)")
	mustExec(t, db, "CREATE TABLE orders (id INTEGER PRIMARY KEY, user_id INTEGER, total INT)")
	mustExec(t, db, "CREATE TABLE order_items (id INTEGER PRIMARY KEY, order_id INTEGER, sku TEXT)")
	mustExec(t, db, "INSERT INTO users (id, name) VALUES (1, 'Alice'), (2, 'Bob')")
	mustExec(t, db, "INSERT INTO orders (id, user_id, total) VALUES (10, 1, 50), (11, 1, 70), (20, 2, 99)")
	mustExec(t, db, "INSERT INTO order_items (id, order_id, sku) VALUES (100, 10, 'A-1'), (101, 11, 'A-2'), (200, 20, 'B-1')")

	cfg := config.Config{
		Driver:          "sqlite",
		OutputDriver:    "postgres",
		DSN:             "unused",
		Table:           "users",
		Column:          "id",
		Record:          "1",
		RelationsByName: true,
	}

	sqlDump, err := Run(context.Background(), db, cfg)
	if err != nil {
		t.Fatalf("erro ao exportar com relações por nome: %v", err)
	}

	containsAll(t, sqlDump,
		`CREATE TABLE IF NOT EXISTS "users"`,
		`CREATE TABLE IF NOT EXISTS "orders"`,
		`CREATE TABLE IF NOT EXISTS "order_items"`,
		`FOREIGN KEY ("user_id") REFERENCES "users" ("id")`,
		`FOREIGN KEY ("order_id") REFERENCES "orders" ("id")`,
		`INSERT INTO "users" ("id", "name") VALUES (1, 'Alice');`,
		`INSERT INTO "orders" ("id", "user_id", "total") VALUES (10, 1, 50);`,
		`INSERT INTO "orders" ("id", "user_id", "total") VALUES (11, 1, 70);`,
		`INSERT INTO "order_items" ("id", "order_id", "sku") VALUES (100, 10, 'A-1');`,
		`INSERT INTO "order_items" ("id", "order_id", "sku") VALUES (101, 11, 'A-2');`,
	)

	if strings.Contains(sqlDump, `'Bob'`) || strings.Contains(sqlDump, "(20, 2, 99)") || strings.Contains(sqlDump, "'B-1'") {
		t.Fatalf("o dump não deveria incluir registros desconectados do grafo inicial: %s", sqlDump)
	}

	assertInOrder(t, sqlDump,
		`CREATE TABLE IF NOT EXISTS "users"`,
		`CREATE TABLE IF NOT EXISTS "orders"`,
		`CREATE TABLE IF NOT EXISTS "order_items"`,
		`INSERT INTO "users"`,
		`INSERT INTO "orders"`,
		`INSERT INTO "order_items"`,
	)
}

func TestRunExportNewIDsComRelacoesPorNomeNoDestinoSQLite(t *testing.T) {
	db := openSQLiteTestDB(t)

	mustExec(t, db, "CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT)")
	mustExec(t, db, "CREATE TABLE orders (id INTEGER PRIMARY KEY, user_id INTEGER, total INT)")
	mustExec(t, db, "CREATE TABLE order_items (id INTEGER PRIMARY KEY, order_id INTEGER, sku TEXT)")
	mustExec(t, db, "INSERT INTO users (id, name) VALUES (1, 'Alice')")
	mustExec(t, db, "INSERT INTO orders (id, user_id, total) VALUES (10, 1, 99)")
	mustExec(t, db, "INSERT INTO order_items (id, order_id, sku) VALUES (100, 10, 'A-1')")

	cfg := config.Config{
		Driver:          "sqlite",
		OutputDriver:    "sqlite",
		DSN:             "unused",
		Table:           "users",
		Column:          "id",
		Record:          "1",
		NewIDs:          true,
		RelationsByName: true,
	}

	sqlDump, err := Run(context.Background(), db, cfg)
	if err != nil {
		t.Fatalf("erro ao exportar com new_ids e relações por nome: %v", err)
	}

	containsAll(t, sqlDump,
		"CREATE TEMP TABLE `__tracesql_map_users`",
		"CREATE TEMP TABLE `__tracesql_map_orders`",
		"CREATE TEMP TABLE `__tracesql_map_order_items`",
		"INSERT INTO `users` (`name`) VALUES ('Alice');",
		"INSERT INTO `__tracesql_map_users` (`old_id`, `new_id`) VALUES (1, last_insert_rowid());",
		"INSERT INTO `orders` (`user_id`, `total`) VALUES ((SELECT `new_id` FROM `__tracesql_map_users` WHERE `old_id` = 1), 99);",
		"INSERT INTO `__tracesql_map_orders` (`old_id`, `new_id`) VALUES (10, last_insert_rowid());",
		"INSERT INTO `order_items` (`order_id`, `sku`) VALUES ((SELECT `new_id` FROM `__tracesql_map_orders` WHERE `old_id` = 10), 'A-1');",
	)
}

func TestRunExportDepthZeroMantemApenasRegistroBase(t *testing.T) {
	db := openSQLiteTestDB(t)

	mustExec(t, db, "PRAGMA foreign_keys = ON")
	mustExec(t, db, "CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT)")
	mustExec(t, db, "CREATE TABLE orders (id INTEGER PRIMARY KEY, user_id INTEGER, total INT, FOREIGN KEY(user_id) REFERENCES users(id))")
	mustExec(t, db, "CREATE TABLE order_items (id INTEGER PRIMARY KEY, order_id INTEGER, sku TEXT, FOREIGN KEY(order_id) REFERENCES orders(id))")
	mustExec(t, db, "INSERT INTO users (id, name) VALUES (1, 'Alice')")
	mustExec(t, db, "INSERT INTO orders (id, user_id, total) VALUES (10, 1, 99)")
	mustExec(t, db, "INSERT INTO order_items (id, order_id, sku) VALUES (100, 10, 'A-1')")

	depth := 0
	cfg := config.Config{
		Driver:       "sqlite",
		OutputDriver: "sqlite",
		DSN:          "unused",
		Table:        "orders",
		Column:       "id",
		Record:       "10",
		Depth:        &depth,
	}

	sqlDump, err := Run(context.Background(), db, cfg)
	if err != nil {
		t.Fatalf("erro ao exportar com depth 0: %v", err)
	}

	containsAll(t, sqlDump,
		"CREATE TABLE IF NOT EXISTS `orders`",
		"INSERT INTO `orders` (`id`, `user_id`, `total`) VALUES (10, 1, 99);",
	)

	if strings.Contains(sqlDump, "CREATE TABLE IF NOT EXISTS `users`") || strings.Contains(sqlDump, "CREATE TABLE IF NOT EXISTS `order_items`") {
		t.Fatalf("depth 0 nao deveria incluir tabelas relacionadas: %s", sqlDump)
	}
	if strings.Contains(sqlDump, "INSERT INTO `users`") || strings.Contains(sqlDump, "INSERT INTO `order_items`") {
		t.Fatalf("depth 0 nao deveria incluir inserts relacionados: %s", sqlDump)
	}
}

func TestRunExportDepthUmIncluiSomentePrimeiroNivel(t *testing.T) {
	db := openSQLiteTestDB(t)

	mustExec(t, db, "PRAGMA foreign_keys = ON")
	mustExec(t, db, "CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT)")
	mustExec(t, db, "CREATE TABLE orders (id INTEGER PRIMARY KEY, user_id INTEGER, total INT, FOREIGN KEY(user_id) REFERENCES users(id))")
	mustExec(t, db, "CREATE TABLE order_items (id INTEGER PRIMARY KEY, order_id INTEGER, sku TEXT, FOREIGN KEY(order_id) REFERENCES orders(id))")
	mustExec(t, db, "CREATE TABLE item_events (id INTEGER PRIMARY KEY, order_item_id INTEGER, action TEXT, FOREIGN KEY(order_item_id) REFERENCES order_items(id))")
	mustExec(t, db, "INSERT INTO users (id, name) VALUES (1, 'Alice')")
	mustExec(t, db, "INSERT INTO orders (id, user_id, total) VALUES (10, 1, 99)")
	mustExec(t, db, "INSERT INTO order_items (id, order_id, sku) VALUES (100, 10, 'A-1')")
	mustExec(t, db, "INSERT INTO item_events (id, order_item_id, action) VALUES (1000, 100, 'packed')")

	depth := 1
	cfg := config.Config{
		Driver:       "sqlite",
		OutputDriver: "sqlite",
		DSN:          "unused",
		Table:        "orders",
		Column:       "id",
		Record:       "10",
		Depth:        &depth,
	}

	sqlDump, err := Run(context.Background(), db, cfg)
	if err != nil {
		t.Fatalf("erro ao exportar com depth 1: %v", err)
	}

	containsAll(t, sqlDump,
		"CREATE TABLE IF NOT EXISTS `users`",
		"CREATE TABLE IF NOT EXISTS `orders`",
		"CREATE TABLE IF NOT EXISTS `order_items`",
		"INSERT INTO `users` (`id`, `name`) VALUES (1, 'Alice');",
		"INSERT INTO `orders` (`id`, `user_id`, `total`) VALUES (10, 1, 99);",
		"INSERT INTO `order_items` (`id`, `order_id`, `sku`) VALUES (100, 10, 'A-1');",
	)

	if strings.Contains(sqlDump, "CREATE TABLE IF NOT EXISTS `item_events`") || strings.Contains(sqlDump, "INSERT INTO `item_events`") {
		t.Fatalf("depth 1 nao deveria incluir o segundo nivel de relacoes: %s", sqlDump)
	}
}

func TestRunExportIgnoraTabelasPorSufixo(t *testing.T) {
	db := openSQLiteTestDB(t)

	mustExec(t, db, "CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT)")
	mustExec(t, db, "CREATE TABLE orders (id INTEGER PRIMARY KEY, user_id INTEGER, total INT)")
	mustExec(t, db, "CREATE TABLE orders_log (id INTEGER PRIMARY KEY, order_id INTEGER, message TEXT)")
	mustExec(t, db, "INSERT INTO users (id, name) VALUES (1, 'Alice')")
	mustExec(t, db, "INSERT INTO orders (id, user_id, total) VALUES (10, 1, 99)")
	mustExec(t, db, "INSERT INTO orders_log (id, order_id, message) VALUES (100, 10, 'created')")

	cfg := config.Config{
		Driver:            "sqlite",
		OutputDriver:      "sqlite",
		DSN:               "unused",
		Table:             "users",
		Column:            "id",
		Record:            "1",
		RelationsByName:   true,
		IgnoreTableSuffix: "_log",
	}

	sqlDump, err := Run(context.Background(), db, cfg)
	if err != nil {
		t.Fatalf("erro ao exportar ignorando sufixo: %v", err)
	}

	containsAll(t, sqlDump,
		"CREATE TABLE IF NOT EXISTS `users`",
		"CREATE TABLE IF NOT EXISTS `orders`",
		"INSERT INTO `users` (`id`, `name`) VALUES (1, 'Alice');",
		"INSERT INTO `orders` (`id`, `user_id`, `total`) VALUES (10, 1, 99);",
	)

	if strings.Contains(sqlDump, "orders_log") || strings.Contains(sqlDump, "'created'") {
		t.Fatalf("tabelas com o sufixo ignorado nao deveriam aparecer no dump: %s", sqlDump)
	}
}

func containsAll(t *testing.T, content string, snippets ...string) {
	t.Helper()

	for _, snippet := range snippets {
		if !strings.Contains(content, snippet) {
			t.Fatalf("trecho não encontrado no dump: %s\n\nDump:\n%s", snippet, content)
		}
	}
}

func assertInOrder(t *testing.T, content string, snippets ...string) {
	t.Helper()

	lastIndex := -1
	for _, snippet := range snippets {
		index := strings.Index(content, snippet)
		if index == -1 {
			t.Fatalf("trecho não encontrado no dump: %s\n\nDump:\n%s", snippet, content)
		}
		if index < lastIndex {
			t.Fatalf("ordem inesperada no dump para %s\n\nDump:\n%s", snippet, content)
		}
		lastIndex = index
	}
}
