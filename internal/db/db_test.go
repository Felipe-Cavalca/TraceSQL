package db_test

import (
	"path/filepath"
	"testing"

	"github.com/Felipe-Cavalca/TraceSQL/internal/db"
)

// O driver SQLite precisa funcionar com os dois nomes aceitos em --driver.
func TestOpenSQLiteExecutaCreateInsertSelect(t *testing.T) {
	for _, driver := range []string{"sqlite", "sqlite3"} {
		t.Run(driver, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "teste.db")

			conn, err := db.Open(driver, path)
			if err != nil {
				t.Fatalf("Open(%q) falhou: %v", driver, err)
			}
			t.Cleanup(func() { _ = conn.Close() })

			if _, err := conn.Exec("CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT)"); err != nil {
				t.Fatalf("CREATE TABLE: %v", err)
			}
			if _, err := conn.Exec("INSERT INTO users (id, name) VALUES (1, 'Alice')"); err != nil {
				t.Fatalf("INSERT: %v", err)
			}

			var name string
			if err := conn.QueryRow("SELECT name FROM users WHERE id = 1").Scan(&name); err != nil {
				t.Fatalf("SELECT: %v", err)
			}
			if name != "Alice" {
				t.Errorf("name = %q, esperado %q", name, "Alice")
			}
		})
	}
}

func TestOpenSQLiteEmMemoria(t *testing.T) {
	conn, err := db.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Open em memória falhou: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	conn.SetMaxOpenConns(1)

	if _, err := conn.Exec("CREATE TABLE t (id INTEGER)"); err != nil {
		t.Fatalf("CREATE TABLE: %v", err)
	}
	if _, err := conn.Exec("INSERT INTO t (id) VALUES (42)"); err != nil {
		t.Fatalf("INSERT: %v", err)
	}
	var id int
	if err := conn.QueryRow("SELECT id FROM t").Scan(&id); err != nil || id != 42 {
		t.Fatalf("SELECT id = %d, err = %v", id, err)
	}
}
