package export

import (
	"context"
	"database/sql"
	"strings"
	"testing"

	"github.com/Felipe-Cavalca/TraceSQL/internal/config"
	_ "github.com/glebarez/sqlite"
)

func TestRunExportPreserveIDs(t *testing.T) {
	db, err := sql.Open("sqlite", "file::memory:?cache=shared")
	if err != nil {
		t.Fatalf("abrindo sqlite: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	_, _ = db.Exec("CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT, age INT)")
	_, _ = db.Exec("INSERT INTO users (id, name, age) VALUES (1, 'Alice', 30)")

	cfg := config.Config{
		Driver: "sqlite",
		DSN:    "file::memory:?cache=shared",
		Table:  "users",
		Column: "id",
		Record: "1",
		NewIDs: false,
	}

	sqlDump, err := Run(context.Background(), db, cfg)
	if err != nil {
		t.Fatalf("erro ao exportar: %v", err)
	}

	if !strings.Contains(sqlDump, "INSERT INTO `users` (`id`, `name`, `age`) VALUES (1, 'Alice', 30);") {
		t.Fatalf("SQL inesperado: %s", sqlDump)
	}
}

func TestRunExportGenerateNewIDs(t *testing.T) {
	db, err := sql.Open("sqlite", "file::memory:?cache=shared")
	if err != nil {
		t.Fatalf("abrindo sqlite: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	_, _ = db.Exec("CREATE TABLE orders (id INTEGER PRIMARY KEY, total INT)")
	_, _ = db.Exec("INSERT INTO orders (id, total) VALUES (10, 99)")

	cfg := config.Config{
		Driver: "sqlite",
		DSN:    "file::memory:?cache=shared",
		Table:  "orders",
		Column: "id",
		Record: "10",
		NewIDs: true,
	}

	sqlDump, err := Run(context.Background(), db, cfg)
	if err != nil {
		t.Fatalf("erro ao exportar: %v", err)
	}

	if strings.Contains(sqlDump, "`id`") {
		t.Fatalf("id n?o deveria aparecer quando new_ids = true: %s", sqlDump)
	}
}
