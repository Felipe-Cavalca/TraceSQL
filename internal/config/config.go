package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

// Config agrupa par?metros de conex?o e exporta??o.
type Config struct {
	Driver  string
	DSN     string
	Table   string
	Column  string
	Record  string
	OutFile string
	NewIDs  bool
}

// Default l? valores do ambiente e define padr?es.
func Default() Config {
	return Config{
		Driver:  getEnv("TRACESQL_DRIVER", ""),
		DSN:     getEnv("TRACESQL_DSN", ""),
		Column:  "id",
		OutFile: getEnv("TRACESQL_OUT", ""),
		NewIDs:  strings.EqualFold(getEnv("TRACESQL_NEW_IDS", "false"), "true"),
	}
}

// AttachFlags registra as flags do CLI.
func AttachFlags(cmd *cobra.Command, cfg Config) {
	cmd.PersistentFlags().String("driver", cfg.Driver, "Driver do banco (postgres, mysql, sqlite)")
	cmd.PersistentFlags().String("dsn", cfg.DSN, "DSN de conex?o do banco")
	cmd.PersistentFlags().String("table", cfg.Table, "Tabela de origem")
	cmd.PersistentFlags().String("column", cfg.Column, "Coluna de refer?ncia (padr?o id)")
	cmd.PersistentFlags().String("record", cfg.Record, "Valor do registro a exportar")
	cmd.PersistentFlags().String("out", cfg.OutFile, "Caminho do arquivo .sql a gerar")
	cmd.PersistentFlags().Bool("new-ids", cfg.NewIDs, "Gerar novos IDs (omite a coluna de refer?ncia no insert)")
}

// BindFlags aplica os valores das flags na config.
func BindFlags(cmd *cobra.Command, cfg *Config) error {
	if v, err := cmd.Flags().GetString("driver"); err == nil && v != "" {
		cfg.Driver = v
	}
	if v, err := cmd.Flags().GetString("dsn"); err == nil && v != "" {
		cfg.DSN = v
	}
	if v, err := cmd.Flags().GetString("table"); err == nil && v != "" {
		cfg.Table = v
	}
	if v, err := cmd.Flags().GetString("column"); err == nil && v != "" {
		cfg.Column = v
	}
	if v, err := cmd.Flags().GetString("record"); err == nil && v != "" {
		cfg.Record = v
	}
	if v, err := cmd.Flags().GetString("out"); err == nil && v != "" {
		cfg.OutFile = v
	}
	if v, err := cmd.Flags().GetBool("new-ids"); err == nil {
		cfg.NewIDs = v
	}
	return nil
}

// Validate garante que campos obrigat?rios foram fornecidos.
func (c Config) Validate() error {
	missing := []string{}
	if c.Driver == "" {
		missing = append(missing, "driver")
	}
	if c.DSN == "" {
		missing = append(missing, "dsn")
	}
	if c.Table == "" {
		missing = append(missing, "table")
	}
	if c.Record == "" {
		missing = append(missing, "record")
	}
	if len(missing) > 0 {
		return fmt.Errorf("campos obrigat?rios faltando: %s", strings.Join(missing, ", "))
	}
	return nil
}

// OutPath resolve o caminho de sa?da.
func (c Config) OutPath() string {
	if c.OutFile != "" {
		return c.OutFile
	}
	if c.Table != "" && c.Record != "" {
		return fmt.Sprintf("export_%s_%s.sql", c.Table, c.Record)
	}
	return "export.sql"
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return fallback
}

// WithPrompts preenche campos faltantes (usado nos testes).
func (c *Config) WithPrompts(values map[string]string) {
	if c.Driver == "" {
		c.Driver = values["driver"]
	}
	if c.DSN == "" {
		c.DSN = values["dsn"]
	}
	if c.Table == "" {
		c.Table = values["table"]
	}
	if c.Column == "" {
		c.Column = "id"
	}
	if c.Record == "" {
		c.Record = values["record"]
	}
}

// Normalize reduz abrevia??es dos drivers.
func (c *Config) Normalize() {
	d := strings.ToLower(c.Driver)
	switch d {
	case "postgres", "postgresql", "pg":
		c.Driver = "postgres"
	case "mysql":
		c.Driver = "mysql"
	case "sqlite", "sqlite3":
		c.Driver = "sqlite"
	}
}

func (c *Config) MergeEnv(env map[string]string) {
	for k, v := range env {
		os.Setenv(k, v)
	}
}

func (c *Config) EnsureDefaults() {
	if c.Column == "" {
		c.Column = "id"
	}
}
