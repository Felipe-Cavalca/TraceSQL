package config_test

import (
	"os"
	"testing"

	"github.com/Felipe-Cavalca/TraceSQL/internal/config"
)

func TestDefaultFromEnv(t *testing.T) {
	t.Setenv("TRACESQL_DRIVER", "postgres")
	t.Setenv("TRACESQL_DSN", "postgres://user:pass@localhost/db")
	t.Setenv("TRACESQL_TABLE", "users")
	t.Setenv("TRACESQL_COLUMN", "uuid")
	t.Setenv("TRACESQL_RECORD", "abc")
	t.Setenv("TRACESQL_NEW_IDS", "true")

	cfg := config.Default()

	if cfg.Driver != "postgres" || cfg.DSN == "" || cfg.Table != "users" {
		t.Fatalf("env n?o aplicado corretamente: %+v", cfg)
	}
	if !cfg.NewIDs {
		t.Fatalf("flag de novos IDs deveria estar true")
	}
}

func TestOutPath(t *testing.T) {
	cfg := config.Config{Table: "orders", Record: "42"}
	if cfg.OutPath() != "export_orders_42.sql" {
		t.Fatalf("OutPath inesperado: %s", cfg.OutPath())
	}

	cfg.OutFile = "custom.sql"
	if cfg.OutPath() != "custom.sql" {
		t.Fatalf("OutPath deveria respeitar OutFile")
	}
}

func TestValidate(t *testing.T) {
	cfg := config.Config{}
	if err := cfg.Validate(); err == nil {
		t.Fatal("deveria falhar sem campos obrigat?rios")
	}

	cfg.Driver = "sqlite"
	cfg.DSN = "file::memory:?cache=shared"
	cfg.Table = "users"
	cfg.Record = "1"
	if err := cfg.Validate(); err != nil {
		t.Fatalf("n?o deveria falhar: %v", err)
	}
}

func TestNormalize(t *testing.T) {
	cfg := config.Config{Driver: "PG"}
	cfg.Normalize()
	if cfg.Driver != "postgres" {
		t.Fatalf("esperado postgres, obtido %s", cfg.Driver)
	}
}

func TestEnsureDefaults(t *testing.T) {
	cfg := config.Config{}
	cfg.EnsureDefaults()
	if cfg.Column != "id" {
		t.Fatalf("coluna padr?o deveria ser id, obtido %s", cfg.Column)
	}

	os.Unsetenv("TRACESQL_COLUMN")
}
