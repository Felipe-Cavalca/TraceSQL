package config_test

import (
	"testing"

	"github.com/Felipe-Cavalca/TraceSQL/internal/config"
)

func TestDefaultFromEnv(t *testing.T) {
	t.Setenv("TRACESQL_DRIVER", "postgres")
	t.Setenv("TRACESQL_OUTPUT_DRIVER", "mysql")
	t.Setenv("TRACESQL_HOST", "127.0.0.1")
	t.Setenv("TRACESQL_PORT", "5432")
	t.Setenv("TRACESQL_USER", "app")
	t.Setenv("TRACESQL_PASSWORD", "secret")
	t.Setenv("TRACESQL_DATABASE", "trace")
	t.Setenv("TRACESQL_NEW_IDS", "true")

	cfg := config.Default()

	if cfg.Driver != "postgres" || cfg.Host != "127.0.0.1" {
		t.Fatalf("env nao aplicado corretamente: %+v", cfg)
	}
	if cfg.OutputDriver != "mysql" {
		t.Fatalf("output driver nao aplicado corretamente: %+v", cfg)
	}
	if cfg.Database != "trace" || cfg.User != "app" || cfg.Password != "secret" {
		t.Fatalf("campos de conexao nao foram carregados: %+v", cfg)
	}
	if cfg.Table != "" || cfg.Record != "" {
		t.Fatalf("table/record nao deveriam vir do env: %+v", cfg)
	}
	if cfg.Column != "id" {
		t.Fatalf("coluna padrao deveria ser id, obtido %s", cfg.Column)
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
		t.Fatal("deveria falhar sem campos obrigatorios")
	}

	cfg = config.Config{
		Driver:   "postgres",
		Host:     "127.0.0.1",
		Port:     "5432",
		User:     "app",
		Database: "trace",
		Table:    "users",
		Record:   "1",
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("nao deveria falhar com conexao em campos separados: %v", err)
	}

	cfg = config.Config{
		Driver:   "sqlite",
		Database: "trace.db",
		Table:    "users",
		Record:   "1",
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("nao deveria falhar para sqlite com database: %v", err)
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
	cfg := config.Config{Driver: "sqlite"}
	cfg.EnsureDefaults()
	if cfg.Column != "id" {
		t.Fatalf("coluna padrao deveria ser id, obtido %s", cfg.Column)
	}
	if cfg.OutputDriver != "sqlite" {
		t.Fatalf("output driver padrao deveria seguir o driver, obtido %s", cfg.OutputDriver)
	}
}

func TestConnectionStringPrefersDSN(t *testing.T) {
	cfg := config.Config{
		Driver: "postgres",
		DSN:    "postgres://user:pass@localhost:5432/app",
	}

	dsn, err := cfg.ConnectionString()
	if err != nil {
		t.Fatalf("connection string: %v", err)
	}
	if dsn != cfg.DSN {
		t.Fatalf("esperado usar a DSN original, obtido %q", dsn)
	}
}

func TestConnectionStringPostgres(t *testing.T) {
	cfg := config.Config{
		Driver:   "postgres",
		Host:     "127.0.0.1",
		Port:     "5432",
		User:     "app",
		Password: "secret",
		Database: "trace",
	}

	dsn, err := cfg.ConnectionString()
	if err != nil {
		t.Fatalf("connection string: %v", err)
	}
	expected := "postgres://app:secret@127.0.0.1:5432/trace"
	if dsn != expected {
		t.Fatalf("dsn inesperada: %s", dsn)
	}
}

func TestConnectionStringMySQL(t *testing.T) {
	cfg := config.Config{
		Driver:   "mysql",
		Host:     "127.0.0.1",
		Port:     "3306",
		User:     "app",
		Password: "secret",
		Database: "trace",
	}

	dsn, err := cfg.ConnectionString()
	if err != nil {
		t.Fatalf("connection string: %v", err)
	}
	expected := "app:secret@tcp(127.0.0.1:3306)/trace"
	if dsn != expected {
		t.Fatalf("dsn inesperada: %s", dsn)
	}
}

func TestConnectionStringSQLite(t *testing.T) {
	cfg := config.Config{
		Driver:   "sqlite",
		Database: "trace.db",
	}

	dsn, err := cfg.ConnectionString()
	if err != nil {
		t.Fatalf("connection string: %v", err)
	}
	if dsn != "trace.db" {
		t.Fatalf("dsn inesperada: %s", dsn)
	}
}
