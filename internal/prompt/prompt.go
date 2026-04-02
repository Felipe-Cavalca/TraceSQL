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
		cfg.Driver = readLine(reader)
	}

	cfg.Normalize()
	cfg.EnsureDefaults()

	if !cfg.DSNProvided() && cfg.DSN == "" {
		switch cfg.Driver {
		case "postgres", "mysql":
			if !cfg.HostProvided() && cfg.Host == "" {
				fmt.Fprint(out, "IP ou host do banco: ")
				cfg.Host = readLine(reader)
			}
			if !cfg.PortProvided() && cfg.Port == "" {
				cfg.Port = promptWithDefault(reader, out, "Porta do banco", cfg.DefaultPort())
			}
			if !cfg.UserProvided() && cfg.User == "" {
				fmt.Fprint(out, "Usuario do banco: ")
				cfg.User = readLine(reader)
			}
			if !cfg.PasswordProvided() && cfg.Password == "" {
				fmt.Fprint(out, "Senha do banco: ")
				cfg.Password = readLine(reader)
			}
			if !cfg.DatabaseProvided() && cfg.Database == "" {
				fmt.Fprint(out, "Nome do banco: ")
				cfg.Database = readLine(reader)
			}
		case "sqlite":
			if !cfg.DatabaseProvided() && cfg.Database == "" {
				fmt.Fprint(out, "Caminho do banco SQLite: ")
				cfg.Database = readLine(reader)
			}
		default:
			fmt.Fprint(out, "DSN de conexao: ")
			cfg.DSN = readLine(reader)
		}
	}

	if !cfg.TableProvided() && cfg.Table == "" {
		fmt.Fprint(out, "Tabela de origem: ")
		cfg.Table = readLine(reader)
	}
	if cfg.Column == "" {
		cfg.Column = "id"
	}
	if !cfg.ColumnProvided() {
		if cfg.Column == "id" {
			fmt.Fprint(out, "Coluna de referencia [id]: ")
		} else {
			fmt.Fprintf(out, "Coluna de referencia [%s]: ", cfg.Column)
		}
		if c := readLine(reader); c != "" {
			cfg.Column = c
		}
	}

	if !cfg.RecordProvided() && cfg.Record == "" {
		fmt.Fprintf(out, "Valor do registro (%s): ", cfg.Column)
		cfg.Record = readLine(reader)
	}

	if !cfg.NewIDsProvided() {
		fmt.Fprint(out, "Gerar novos IDs? (s/N): ")
		resp := strings.ToLower(readLine(reader))
		cfg.NewIDs = resp == "s" || resp == "y" || resp == "sim"
	}

	cfg.Normalize()
	cfg.EnsureDefaults()
	return nil
}

func readLine(reader *bufio.Reader) string {
	value, _ := reader.ReadString('\n')
	return strings.TrimSpace(value)
}

func promptWithDefault(reader *bufio.Reader, out io.Writer, label, fallback string) string {
	if fallback == "" {
		fmt.Fprintf(out, "%s: ", label)
		return readLine(reader)
	}

	fmt.Fprintf(out, "%s [%s]: ", label, fallback)
	if value := readLine(reader); value != "" {
		return value
	}
	return fallback
}
