package export

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"testing"

	"github.com/Felipe-Cavalca/TraceSQL/internal/config"
	_ "github.com/glebarez/sqlite"
)

func TestRunExportPreserveIDs(t *testing.T) {
	db := openSQLiteTestDB(t)

	mustExec(t, db, "CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT, age INT)")
	mustExec(t, db, "INSERT INTO users (id, name, age) VALUES (1, 'Alice', 30)")

	cfg := config.Config{
		Driver:       "sqlite",
		OutputDriver: "sqlite",
		DSN:          "unused",
		Table:        "users",
		Column:       "id",
		Record:       "1",
		NewIDs:       false,
	}

	sqlDump, err := Run(context.Background(), db, cfg)
	if err != nil {
		t.Fatalf("erro ao exportar: %v", err)
	}

	if !strings.Contains(sqlDump, "CREATE TABLE IF NOT EXISTS `users`") {
		t.Fatalf("schema da tabela users não encontrado: %s", sqlDump)
	}
	if !strings.Contains(sqlDump, "INSERT INTO `users` (`id`, `name`, `age`) VALUES (1, 'Alice', 30);") {
		t.Fatalf("insert da tabela users não encontrado: %s", sqlDump)
	}
	if strings.Index(sqlDump, "CREATE TABLE IF NOT EXISTS `users`") > strings.Index(sqlDump, "INSERT INTO `users`") {
		t.Fatalf("schema deveria aparecer antes do insert: %s", sqlDump)
	}
}

func TestRunExportGenerateNewIDs(t *testing.T) {
	db := openSQLiteTestDB(t)

	mustExec(t, db, "CREATE TABLE orders (id INTEGER PRIMARY KEY, total INT)")
	mustExec(t, db, "INSERT INTO orders (id, total) VALUES (10, 99)")

	cfg := config.Config{
		Driver:       "sqlite",
		OutputDriver: "sqlite",
		DSN:          "unused",
		Table:        "orders",
		Column:       "id",
		Record:       "10",
		NewIDs:       true,
	}

	sqlDump, err := Run(context.Background(), db, cfg)
	if err != nil {
		t.Fatalf("erro ao exportar: %v", err)
	}

	if !strings.Contains(sqlDump, "INSERT INTO `orders` (`total`) VALUES (99);") {
		t.Fatalf("insert deveria omitir a coluna id quando new_ids = true: %s", sqlDump)
	}
	if strings.Contains(sqlDump, "INSERT INTO `orders` (`id`,") {
		t.Fatalf("id não deveria aparecer no insert quando new_ids = true: %s", sqlDump)
	}
}

func openSQLiteTestDB(t *testing.T) *sql.DB {
	t.Helper()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.NewReplacer("/", "_", " ", "_").Replace(t.Name()))
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatalf("abrindo sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func mustExec(t *testing.T, db *sql.DB, query string) {
	t.Helper()

	if _, err := db.Exec(query); err != nil {
		t.Fatalf("falha ao executar %q: %v", query, err)
	}
}
