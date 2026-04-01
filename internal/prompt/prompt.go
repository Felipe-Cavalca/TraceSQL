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

	if !cfg.DriverProvided() && cfg.Driver == "" {
		fmt.Fprint(out, "Driver (postgres/mysql/sqlite): ")
		d, _ := reader.ReadString('\n')
		cfg.Driver = strings.TrimSpace(d)
	}
	if !cfg.DSNProvided() && cfg.DSN == "" {
		fmt.Fprint(out, "DSN de conex?o: ")
		d, _ := reader.ReadString('\n')
		cfg.DSN = strings.TrimSpace(d)
	}
	if !cfg.TableProvided() && cfg.Table == "" {
		fmt.Fprint(out, "Tabela de origem: ")
		d, _ := reader.ReadString('\n')
		cfg.Table = strings.TrimSpace(d)
	}
	if cfg.Column == "" {
		cfg.Column = "id"
	}
	if !cfg.ColumnProvided() {
		if cfg.Column == "id" {
			fmt.Fprintf(out, "Coluna de refer?ncia [id]: ")
		} else {
			fmt.Fprintf(out, "Coluna de refer?ncia [%s]: ", cfg.Column)
		}
		if c, _ := reader.ReadString('\n'); strings.TrimSpace(c) != "" {
			cfg.Column = strings.TrimSpace(c)
		}
	}

	if !cfg.RecordProvided() && cfg.Record == "" {
		fmt.Fprintf(out, "Valor do registro (%s): ", cfg.Column)
		d, _ := reader.ReadString('\n')
		cfg.Record = strings.TrimSpace(d)
	}

	if !cfg.NewIDsProvided() {
		fmt.Fprint(out, "Gerar novos IDs? (s/N): ")
		resp, _ := reader.ReadString('\n')
		resp = strings.TrimSpace(strings.ToLower(resp))
		cfg.NewIDs = resp == "s" || resp == "y" || resp == "sim"
	}

	cfg.Normalize()
	cfg.EnsureDefaults()
	return nil
}
