package config

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"

	mysql "github.com/go-sql-driver/mysql"
	"github.com/spf13/cobra"
)

// Config agrupa parametros de conexao e exportacao.
type Config struct {
	Driver            string
	OutputDriver      string
	DSN               string
	Host              string
	Port              string
	User              string
	Password          string
	Database          string
	Table             string
	Column            string
	Record            string
	OutFile           string
	NewIDs            bool
	RelationsByName   bool
	Depth             *int
	IgnoreTableSuffix string
	Log               bool

	driverSet          bool
	outputDriverSet    bool
	dsnSet             bool
	hostSet            bool
	portSet            bool
	userSet            bool
	passwordSet        bool
	databaseSet        bool
	tableSet           bool
	columnSet          bool
	recordSet          bool
	outFileSet         bool
	newIDsSet          bool
	relationsByNameSet bool
	logSet             bool
}

// Default le valores do ambiente e define padroes.
func Default() Config {
	driver, driverSet := lookupEnv("TRACESQL_DRIVER")
	outputDriver, outputDriverSet := lookupEnv("TRACESQL_OUTPUT_DRIVER")
	dsn, dsnSet := lookupEnv("TRACESQL_DSN")
	host, hostSet := lookupEnv("TRACESQL_HOST")
	port, portSet := lookupEnv("TRACESQL_PORT")
	user, userSet := lookupEnv("TRACESQL_USER")
	password, passwordSet := lookupEnv("TRACESQL_PASSWORD")
	database, databaseSet := lookupEnv("TRACESQL_DATABASE")
	outFile, outFileSet := lookupEnv("TRACESQL_OUT")
	newIDsRaw, newIDsSet := lookupEnv("TRACESQL_NEW_IDS")
	relationsByNameRaw, relationsByNameSet := lookupEnv("TRACESQL_RELATIONS_BY_NAME")
	depthRaw, _ := lookupEnv("TRACESQL_DEPTH")
	ignoreTableSuffix, _ := lookupEnv("TRACESQL_IGNORE_TABLE_SUFFIX")
	logRaw, logSet := lookupEnv("TRACESQL_LOG")

	return Config{
		Driver:             driver,
		OutputDriver:       outputDriver,
		DSN:                dsn,
		Host:               host,
		Port:               port,
		User:               user,
		Password:           password,
		Database:           database,
		Column:             "id",
		OutFile:            outFile,
		NewIDs:             parseBool(newIDsRaw),
		RelationsByName:    parseBool(relationsByNameRaw),
		Depth:              parseOptionalInt(depthRaw),
		IgnoreTableSuffix:  strings.TrimSpace(ignoreTableSuffix),
		Log:                parseBool(logRaw),
		driverSet:          driverSet,
		outputDriverSet:    outputDriverSet,
		dsnSet:             dsnSet,
		hostSet:            hostSet,
		portSet:            portSet,
		userSet:            userSet,
		passwordSet:        passwordSet,
		databaseSet:        databaseSet,
		outFileSet:         outFileSet,
		newIDsSet:          newIDsSet,
		relationsByNameSet: relationsByNameSet,
		logSet:             logSet,
	}
}

// AttachFlags registra as flags do CLI.
func AttachFlags(cmd *cobra.Command, cfg Config) {
	depthDefault := 0
	if cfg.Depth != nil {
		depthDefault = *cfg.Depth
	}

	cmd.PersistentFlags().String("driver", cfg.Driver, "Driver do banco (postgres, mysql, sqlite)")
	cmd.PersistentFlags().String("output-driver", cfg.OutputDriver, "Dialeto do SQL gerado (padrao: mesmo driver da origem)")
	cmd.PersistentFlags().String("dsn", cfg.DSN, "DSN de conexao do banco (opcional se host/port/user/password/database forem informados)")
	cmd.PersistentFlags().String("host", cfg.Host, "IP ou host do banco")
	cmd.PersistentFlags().String("port", cfg.Port, "Porta do banco")
	cmd.PersistentFlags().String("user", cfg.User, "Usuario do banco")
	cmd.PersistentFlags().String("password", cfg.Password, "Senha do banco")
	cmd.PersistentFlags().String("database", cfg.Database, "Nome do banco ou caminho do arquivo SQLite")
	cmd.PersistentFlags().String("table", cfg.Table, "Tabela de origem")
	cmd.PersistentFlags().String("column", cfg.Column, "Coluna de referencia (padrao id)")
	cmd.PersistentFlags().String("record", cfg.Record, "Valor do registro a exportar")
	cmd.PersistentFlags().String("out", cfg.OutFile, "Caminho do arquivo .sql a gerar")
	cmd.PersistentFlags().Bool("new-ids", cfg.NewIDs, "Gerar novos IDs (omite a coluna de referencia no insert)")
	cmd.PersistentFlags().Bool("relations-by-name", cfg.RelationsByName, "Infere relacoes pelo padrao [tabela]_id quando nao houver foreign key")
	cmd.PersistentFlags().Int("depth", depthDefault, "Profundidade maxima das relacoes (omitido = ilimitado, 0 = so o registro base)")
	cmd.PersistentFlags().String("ignore-table-suffix", cfg.IgnoreTableSuffix, "Ignora tabelas cujo nome termina com este sufixo (ex.: _log)")
	cmd.PersistentFlags().Bool("log", cfg.Log, "Exibe logs de execucao no stderr")
}

// BindFlags aplica os valores das flags na config.
func BindFlags(cmd *cobra.Command, cfg *Config) error {
	flags := cmd.Flags()

	if flags.Changed("driver") {
		cfg.driverSet = true
	}
	cfg.Driver, _ = flags.GetString("driver")

	if flags.Changed("output-driver") {
		cfg.outputDriverSet = true
	}
	cfg.OutputDriver, _ = flags.GetString("output-driver")

	if flags.Changed("dsn") {
		cfg.dsnSet = true
	}
	cfg.DSN, _ = flags.GetString("dsn")

	if flags.Changed("host") {
		cfg.hostSet = true
	}
	cfg.Host, _ = flags.GetString("host")

	if flags.Changed("port") {
		cfg.portSet = true
	}
	cfg.Port, _ = flags.GetString("port")

	if flags.Changed("user") {
		cfg.userSet = true
	}
	cfg.User, _ = flags.GetString("user")

	if flags.Changed("password") {
		cfg.passwordSet = true
	}
	cfg.Password, _ = flags.GetString("password")

	if flags.Changed("database") {
		cfg.databaseSet = true
	}
	cfg.Database, _ = flags.GetString("database")

	if flags.Changed("table") {
		cfg.tableSet = true
	}
	cfg.Table, _ = flags.GetString("table")

	if flags.Changed("column") {
		cfg.columnSet = true
	}
	cfg.Column, _ = flags.GetString("column")

	if flags.Changed("record") {
		cfg.recordSet = true
	}
	cfg.Record, _ = flags.GetString("record")

	if flags.Changed("out") {
		cfg.outFileSet = true
	}
	cfg.OutFile, _ = flags.GetString("out")

	if flags.Changed("new-ids") {
		cfg.newIDsSet = true
	}
	cfg.NewIDs, _ = flags.GetBool("new-ids")

	if flags.Changed("relations-by-name") {
		cfg.relationsByNameSet = true
	}
	cfg.RelationsByName, _ = flags.GetBool("relations-by-name")

	if flags.Changed("depth") {
		depth, _ := flags.GetInt("depth")
		cfg.Depth = &depth
	}

	cfg.IgnoreTableSuffix, _ = flags.GetString("ignore-table-suffix")

	if flags.Changed("log") {
		cfg.logSet = true
	}
	cfg.Log, _ = flags.GetBool("log")

	return nil
}

// Validate garante que campos obrigatorios foram fornecidos.
func (c Config) Validate() error {
	c.Normalize()

	missing := []string{}
	if c.Driver == "" {
		missing = append(missing, "driver")
	}
	if c.Table == "" {
		missing = append(missing, "table")
	}
	if c.Record == "" {
		missing = append(missing, "record")
	}

	if strings.TrimSpace(c.DSN) == "" {
		switch c.Driver {
		case "postgres", "mysql":
			if c.Host == "" {
				missing = append(missing, "host")
			}
			if c.Port == "" {
				missing = append(missing, "port")
			}
			if c.User == "" {
				missing = append(missing, "user")
			}
			if c.Database == "" {
				missing = append(missing, "database")
			}
		case "sqlite":
			if c.Database == "" {
				missing = append(missing, "database")
			}
		default:
			missing = append(missing, "dsn")
		}
	}

	if len(missing) > 0 {
		return fmt.Errorf("campos obrigatorios faltando: %s", strings.Join(missing, ", "))
	}
	if c.Depth != nil && *c.Depth < 0 {
		return fmt.Errorf("depth deve ser maior ou igual a 0 quando informado")
	}
	return nil
}

// OutPath resolve o caminho de saida.
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
	if c.Host == "" {
		c.Host = values["host"]
		c.hostSet = c.Host != ""
	}
	if c.Port == "" {
		c.Port = values["port"]
		c.portSet = c.Port != ""
	}
	if c.User == "" {
		c.User = values["user"]
		c.userSet = c.User != ""
	}
	if c.Password == "" {
		c.Password = values["password"]
		c.passwordSet = c.Password != ""
	}
	if c.Database == "" {
		c.Database = values["database"]
		c.databaseSet = c.Database != ""
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

// Normalize reduz abreviacoes dos drivers.
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

func (c Config) HostProvided() bool {
	return c.hostSet
}

func (c Config) PortProvided() bool {
	return c.portSet
}

func (c Config) UserProvided() bool {
	return c.userSet
}

func (c Config) PasswordProvided() bool {
	return c.passwordSet
}

func (c Config) DatabaseProvided() bool {
	return c.databaseSet
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

func (c Config) DepthLimit() (int, bool) {
	if c.Depth == nil {
		return 0, false
	}
	return *c.Depth, true
}

func (c Config) DepthLabel() string {
	if c.Depth == nil {
		return "ilimitada"
	}
	return strconv.Itoa(*c.Depth)
}

func (c Config) ShouldIgnoreTable(table string) bool {
	suffix := strings.ToLower(strings.TrimSpace(c.IgnoreTableSuffix))
	if suffix == "" {
		return false
	}
	return strings.HasSuffix(strings.ToLower(strings.TrimSpace(table)), suffix)
}

func (c Config) DefaultPort() string {
	switch strings.ToLower(c.Driver) {
	case "postgres":
		return "5432"
	case "mysql":
		return "3306"
	default:
		return ""
	}
}

// ConnectionString retorna a DSN final usada para abrir a conexao.
func (c Config) ConnectionString() (string, error) {
	c.Normalize()

	if strings.TrimSpace(c.DSN) != "" {
		return c.DSN, nil
	}

	switch c.Driver {
	case "postgres":
		return buildPostgresDSN(c), nil
	case "mysql":
		return buildMySQLDSN(c), nil
	case "sqlite":
		if c.Database == "" {
			return "", fmt.Errorf("informe o caminho do banco sqlite em --database ou --dsn")
		}
		return c.Database, nil
	default:
		return "", fmt.Errorf("driver nao suportado: %s", c.Driver)
	}
}

func buildPostgresDSN(c Config) string {
	u := &url.URL{
		Scheme: "postgres",
		Host:   net.JoinHostPort(c.Host, c.Port),
		Path:   "/" + strings.TrimPrefix(c.Database, "/"),
	}
	if c.Password == "" {
		u.User = url.User(c.User)
	} else {
		u.User = url.UserPassword(c.User, c.Password)
	}
	return u.String()
}

func buildMySQLDSN(c Config) string {
	cfg := mysql.NewConfig()
	cfg.Net = "tcp"
	cfg.Addr = net.JoinHostPort(c.Host, c.Port)
	cfg.User = c.User
	cfg.Passwd = c.Password
	cfg.DBName = c.Database
	return cfg.FormatDSN()
}

func parseBool(raw string) bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "1", "t", "true", "y", "yes", "s", "sim":
		return true
	default:
		return false
	}
}

func parseOptionalInt(raw string) *int {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}

	value, err := strconv.Atoi(raw)
	if err != nil {
		invalid := -1
		return &invalid
	}

	return &value
}
