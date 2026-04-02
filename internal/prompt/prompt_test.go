package prompt

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Felipe-Cavalca/TraceSQL/internal/config"
	"github.com/spf13/cobra"
)

func TestFillMissingNaoPerguntaCamposJaInformadosPorFlag(t *testing.T) {
	cfg := config.Default()
	cmd := &cobra.Command{Use: "tracesql"}
	config.AttachFlags(cmd, cfg)
	args := []string{
		"--driver", "mysql",
		"--host", "127.0.0.1",
		"--port", "3307",
		"--user", "tracesql",
		"--password", "tracesql",
		"--database", "tracesql_rich_test",
		"--table", "orders",
		"--column", "id",
		"--record", "1001",
		"--new-ids=false",
		"--output-driver", "sqlite",
		"--log",
	}
	cmd.SetArgs(args)

	if err := cmd.ParseFlags(args); err != nil {
		t.Fatalf("parse flags: %v", err)
	}
	if err := config.BindFlags(cmd, &cfg); err != nil {
		t.Fatalf("bind flags: %v", err)
	}

	var in bytes.Buffer
	var out bytes.Buffer
	if err := FillMissing(&cfg, &in, &out); err != nil {
		t.Fatalf("fill missing: %v", err)
	}

	if out.Len() != 0 {
		t.Fatalf("nao deveria ter pedido prompts, mas escreveu: %q", out.String())
	}
	if cfg.Column != "id" {
		t.Fatalf("column inesperada: %s", cfg.Column)
	}
	if cfg.NewIDs {
		t.Fatalf("new_ids deveria permanecer false")
	}
	if !cfg.Log {
		t.Fatalf("log deveria permanecer true")
	}
}

func TestFillMissingPerguntaCamposDeConexaoSeparados(t *testing.T) {
	cfg := config.Default()

	input := strings.Join([]string{
		"postgres",
		"127.0.0.1",
		"",
		"app_user",
		"secret",
		"trace_db",
		"orders",
		"",
		"1001",
		"s",
	}, "\n") + "\n"

	var in bytes.Buffer
	in.WriteString(input)
	var out bytes.Buffer

	if err := FillMissing(&cfg, &in, &out); err != nil {
		t.Fatalf("fill missing: %v", err)
	}

	if cfg.Driver != "postgres" {
		t.Fatalf("driver inesperado: %s", cfg.Driver)
	}
	if cfg.Host != "127.0.0.1" || cfg.Port != "5432" {
		t.Fatalf("conexao inesperada: host=%s port=%s", cfg.Host, cfg.Port)
	}
	if cfg.User != "app_user" || cfg.Password != "secret" || cfg.Database != "trace_db" {
		t.Fatalf("credenciais inesperadas: %+v", cfg)
	}
	if cfg.Table != "orders" || cfg.Column != "id" || cfg.Record != "1001" {
		t.Fatalf("dados de exportacao inesperados: %+v", cfg)
	}
	if !cfg.NewIDs {
		t.Fatalf("new_ids deveria ser true")
	}

	written := out.String()
	for _, expected := range []string{
		"Driver (postgres/mysql/sqlite): ",
		"IP ou host do banco: ",
		"Porta do banco [5432]: ",
		"Usuario do banco: ",
		"Senha do banco: ",
		"Nome do banco: ",
		"Tabela de origem: ",
		"Coluna de referencia [id]: ",
		"Valor do registro (id): ",
		"Gerar novos IDs? (s/N): ",
	} {
		if !strings.Contains(written, expected) {
			t.Fatalf("prompt nao encontrado: %q em %q", expected, written)
		}
	}
}
