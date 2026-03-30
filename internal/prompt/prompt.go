package prompt

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"github.com/Felipe-Cavalca/TraceSQL/internal/config"
)

// FillMissing pergunta interativamente pelos campos faltantes.
func FillMissing(cfg *config.Config, in io.Reader, out io.Writer) error {
	reader := bufio.NewReader(in)

	cfg.Normalize()
	cfg.EnsureDefaults()

	if cfg.Driver == "" {
		fmt.Fprint(out, "Driver (postgres/mysql/sqlite): ")
		d, _ := reader.ReadString('\n')
		cfg.Driver = strings.TrimSpace(d)
	}
	if cfg.DSN == "" {
		fmt.Fprint(out, "DSN de conex?o: ")
		d, _ := reader.ReadString('\n')
		cfg.DSN = strings.TrimSpace(d)
	}
	if cfg.Table == "" {
		fmt.Fprint(out, "Tabela de origem: ")
		d, _ := reader.ReadString('\n')
		cfg.Table = strings.TrimSpace(d)
	}
	if cfg.Column == "" {
		cfg.Column = "id"
	}
	if cfg.Column == "id" {
		fmt.Fprintf(out, "Coluna de refer?ncia [id]: ")
	} else {
		fmt.Fprintf(out, "Coluna de refer?ncia [%s]: ", cfg.Column)
	}
	if c, _ := reader.ReadString('\n'); strings.TrimSpace(c) != "" {
		cfg.Column = strings.TrimSpace(c)
	}

	if cfg.Record == "" {
		fmt.Fprintf(out, "Valor do registro (%s): ", cfg.Column)
		d, _ := reader.ReadString('\n')
		cfg.Record = strings.TrimSpace(d)
	}

	if !cfg.NewIDs {
		fmt.Fprint(out, "Gerar novos IDs? (s/N): ")
		resp, _ := reader.ReadString('\n')
		resp = strings.TrimSpace(strings.ToLower(resp))
		cfg.NewIDs = resp == "s" || resp == "y" || resp == "sim"
	}

	cfg.Normalize()
	return nil
}
