package prompt

import (
	"bytes"
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
		"--dsn", "tracesql:tracesql@tcp(127.0.0.1:3307)/tracesql_rich_test",
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
		t.Fatalf("não deveria ter pedido prompts, mas escreveu: %q", out.String())
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
