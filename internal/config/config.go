package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

// Config agrupa par?metros de conex?o e exporta??o.
type Config struct {
	Driver       string
	OutputDriver string
	DSN          string
	Table        string
	Column       string
	Record       string
	OutFile      string
	NewIDs       bool
	Log          bool

	driverSet       bool
	outputDriverSet bool
	dsnSet          bool
	tableSet        bool
	columnSet       bool
	recordSet       bool
	outFileSet      bool
	newIDsSet       bool
	logSet          bool
}

// Default l? valores do ambiente e define padr?es.
func Default() Config {
	driver, driverSet := lookupEnv("TRACESQL_DRIVER")
	outputDriver, outputDriverSet := lookupEnv("TRACESQL_OUTPUT_DRIVER")
	dsn, dsnSet := lookupEnv("TRACESQL_DSN")
	outFile, outFileSet := lookupEnv("TRACESQL_OUT")
	newIDsRaw, newIDsSet := lookupEnv("TRACESQL_NEW_IDS")
	logRaw, logSet := lookupEnv("TRACESQL_LOG")

	return Config{
		Driver:          driver,
		OutputDriver:    outputDriver,
		DSN:             dsn,
		Column:          "id",
		OutFile:         outFile,
		NewIDs:          strings.EqualFold(newIDsRaw, "true"),
		Log:             parseBool(logRaw),
		driverSet:       driverSet,
		outputDriverSet: outputDriverSet,
		dsnSet:          dsnSet,
		outFileSet:      outFileSet,
		newIDsSet:       newIDsSet,
		logSet:          logSet,
	}
}

// AttachFlags registra as flags do CLI.
func AttachFlags(cmd *cobra.Command, cfg Config) {
	cmd.PersistentFlags().String("driver", cfg.Driver, "Driver do banco (postgres, mysql, sqlite)")
	cmd.PersistentFlags().String("output-driver", cfg.OutputDriver, "Dialeto do SQL gerado (padrao: mesmo driver da origem)")
	cmd.PersistentFlags().String("dsn", cfg.DSN, "DSN de conex?o do banco")
	cmd.PersistentFlags().String("table", cfg.Table, "Tabela de origem")
	cmd.PersistentFlags().String("column", cfg.Column, "Coluna de refer?ncia (padr?o id)")
	cmd.PersistentFlags().String("record", cfg.Record, "Valor do registro a exportar")
	cmd.PersistentFlags().String("out", cfg.OutFile, "Caminho do arquivo .sql a gerar")
	cmd.PersistentFlags().Bool("new-ids", cfg.NewIDs, "Gerar novos IDs (omite a coluna de refer?ncia no insert)")
	cmd.PersistentFlags().Bool("log", cfg.Log, "Exibe logs de execução no stderr")
}

// BindFlags aplica os valores das flags na config.
func BindFlags(cmd *cobra.Command, cfg *Config) error {
	flags := cmd.Flags()

	if flags.Changed("driver") {
		cfg.driverSet = true
	}
	if v, err := flags.GetString("driver"); err == nil && v != "" {
		cfg.Driver = v
	}
	if flags.Changed("output-driver") {
		cfg.outputDriverSet = true
	}
	if v, err := flags.GetString("output-driver"); err == nil && v != "" {
		cfg.OutputDriver = v
	}
	if flags.Changed("dsn") {
		cfg.dsnSet = true
	}
	if v, err := flags.GetString("dsn"); err == nil && v != "" {
		cfg.DSN = v
	}
	if flags.Changed("table") {
		cfg.tableSet = true
	}
	if v, err := flags.GetString("table"); err == nil && v != "" {
		cfg.Table = v
	}
	if flags.Changed("column") {
		cfg.columnSet = true
	}
	if v, err := flags.GetString("column"); err == nil && v != "" {
		cfg.Column = v
	}
	if flags.Changed("record") {
		cfg.recordSet = true
	}
	if v, err := flags.GetString("record"); err == nil && v != "" {
		cfg.Record = v
	}
	if flags.Changed("out") {
		cfg.outFileSet = true
	}
	if v, err := flags.GetString("out"); err == nil && v != "" {
		cfg.OutFile = v
	}
	if flags.Changed("new-ids") {
		cfg.newIDsSet = true
	}
	if v, err := flags.GetBool("new-ids"); err == nil {
		cfg.NewIDs = v
	}
	if flags.Changed("log") {
		cfg.logSet = true
	}
	if v, err := flags.GetBool("log"); err == nil {
		cfg.Log = v
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

func lookupEnv(key string) (string, bool) {
	return os.LookupEnv(key)
}

func getEnv(key, fallback string) string {
	if v, ok := lookupEnv(key); ok {
		return v
	}
	return fallback
}

// WithPrompts preenche campos faltantes (usado nos testes).
func (c *Config) WithPrompts(values map[string]string) {
	if c.Driver == "" {
		c.Driver = values["driver"]
		c.driverSet = c.Driver != ""
	}
	if c.DSN == "" {
		c.DSN = values["dsn"]
		c.dsnSet = c.DSN != ""
	}
	if c.Table == "" {
		c.Table = values["table"]
		c.tableSet = c.Table != ""
	}
	if c.Column == "" {
		c.Column = "id"
	}
	if c.Record == "" {
		c.Record = values["record"]
		c.recordSet = c.Record != ""
	}
}

// Normalize reduz abrevia??es dos drivers.
func (c *Config) Normalize() {
	switch strings.ToLower(c.Driver) {
	case "postgres", "postgresql", "pg":
		c.Driver = "postgres"
	case "mysql":
		c.Driver = "mysql"
	case "sqlite", "sqlite3":
		c.Driver = "sqlite"
	}

	switch strings.ToLower(c.OutputDriver) {
	case "postgres", "postgresql", "pg":
		c.OutputDriver = "postgres"
	case "mysql":
		c.OutputDriver = "mysql"
	case "sqlite", "sqlite3":
		c.OutputDriver = "sqlite"
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
	if c.OutputDriver == "" && c.Driver != "" {
		c.OutputDriver = c.Driver
	}
}

func (c Config) DriverProvided() bool {
	return c.driverSet
}

func (c Config) DSNProvided() bool {
	return c.dsnSet
}

func (c Config) TableProvided() bool {
	return c.tableSet
}

func (c Config) ColumnProvided() bool {
	return c.columnSet
}

func (c Config) RecordProvided() bool {
	return c.recordSet
}

func (c Config) NewIDsProvided() bool {
	return c.newIDsSet
}

func parseBool(raw string) bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "1", "t", "true", "y", "yes", "s", "sim":
		return true
	default:
		return false
	}
}
