package config_test

import (
	"testing"

	"github.com/spf13/cobra"

	"github.com/Felipe-Cavalca/TraceSQL/internal/config"
)

func TestProgressEnabledRegrasDeSupressao(t *testing.T) {
	cases := []struct {
		nome string
		cfg  config.Config
		tty  bool
		want bool
	}{
		{"padrao em terminal", config.Config{}, true, true},
		{"sem terminal (pipe/CI)", config.Config{}, false, false},
		{"quiet", config.Config{Quiet: true}, true, false},
		{"no-progress", config.Config{NoProgress: true}, true, false},
		{"log ligado", config.Config{Log: true}, true, false},
		{"tudo junto", config.Config{Quiet: true, NoProgress: true, Log: true}, true, false},
	}
	for _, c := range cases {
		if got := c.cfg.ProgressEnabled(c.tty); got != c.want {
			t.Errorf("%s: ProgressEnabled = %t, esperado %t", c.nome, got, c.want)
		}
	}
}

func TestQuietSuprimeLogEInformativas(t *testing.T) {
	if !(config.Config{Log: true}).LogEnabled() {
		t.Error("--log sozinho deveria habilitar o log")
	}
	if (config.Config{Log: true, Quiet: true}).LogEnabled() {
		t.Error("--quiet deveria ter prioridade sobre --log")
	}
	if (config.Config{Quiet: true}).InfoEnabled() {
		t.Error("--quiet deveria suprimir mensagens informativas")
	}
	if !(config.Config{NoProgress: true}).InfoEnabled() {
		t.Error("--no-progress nao deveria suprimir mensagens informativas")
	}
}

func TestNoProgressEQuietPorEnv(t *testing.T) {
	cfg := config.Default()
	if cfg.NoProgress || cfg.Quiet {
		t.Fatalf("padrao deveria manter o progresso ligado: %+v", cfg)
	}

	t.Setenv("TRACESQL_NO_PROGRESS", "true")
	t.Setenv("TRACESQL_QUIET", "1")
	cfg = config.Default()
	if !cfg.NoProgress || !cfg.Quiet {
		t.Fatalf("env nao aplicado: %+v", cfg)
	}
}

func TestNoProgressEQuietPorFlag(t *testing.T) {
	cmd := &cobra.Command{Use: "x"}
	config.AttachFlags(cmd, config.Default())
	if err := cmd.ParseFlags([]string{"--no-progress", "--quiet"}); err != nil {
		t.Fatal(err)
	}
	cfg := config.Default()
	if err := config.BindFlags(cmd, &cfg); err != nil {
		t.Fatal(err)
	}
	if !cfg.NoProgress || !cfg.Quiet {
		t.Fatalf("flags nao aplicadas: %+v", cfg)
	}
}
