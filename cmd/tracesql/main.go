package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/Felipe-Cavalca/TraceSQL/internal/config"
	"github.com/Felipe-Cavalca/TraceSQL/internal/db"
	"github.com/Felipe-Cavalca/TraceSQL/internal/export"
)

type options struct {
	EnvPath string
	Driver  string
	DSN     string
	Table   string
	Column  string
	Output  string
}

func main() {
	log.SetFlags(0)

	var opt options
	flag.StringVar(&opt.EnvPath, "env", ".env", "caminho do arquivo .env")
	flag.StringVar(&opt.Driver, "driver", "", "driver do banco: mysql | postgres | sqlite")
	flag.StringVar(&opt.DSN, "dsn", "", "DSN de conexão (ou defina TRACE_DSN no .env)")
	flag.StringVar(&opt.Table, "table", "", "tabela de origem")
	flag.StringVar(&opt.Column, "column", "id", "coluna de referência (padrão: id)")
	flag.StringVar(&opt.Output, "output", "export.sql", "arquivo de saída SQL")
	flag.Parse()

	_ = config.LoadEnv(opt.EnvPath) // se não existir, apenas ignora

	prompt := bufio.NewReader(os.Stdin)

	opt.Driver = firstNonEmpty(opt.Driver, os.Getenv("TRACE_DRIVER"))
	if opt.Driver == "" {
		opt.Driver = ask(prompt, "Driver (mysql|postgres|sqlite): ")
	}
	if err := config.ValidateDriver(opt.Driver); err != nil {
		log.Fatalf("driver inválido: %v", err)
	}

	opt.DSN = firstNonEmpty(opt.DSN, os.Getenv("TRACE_DSN"))
	if opt.DSN == "" {
		opt.DSN = ask(prompt, "DSN de conexão: ")
	}

	opt.Table = firstNonEmpty(opt.Table, os.Getenv("TRACE_TABLE"))
	if opt.Table == "" {
		opt.Table = ask(prompt, "Tabela de origem: ")
	}

	opt.Column = firstNonEmpty(opt.Column, os.Getenv("TRACE_COLUMN"))
	if opt.Column == "" {
		opt.Column = "id"
	}

	log.Printf("Conectando via driver %s...", opt.Driver)
	conn, closer, err := db.Connect(opt.Driver, opt.DSN)
	if err != nil {
		log.Fatalf("falha ao conectar: %v", err)
	}
	defer closer()

	if err := conn.Ping(); err != nil {
		log.Fatalf("ping falhou: %v", err)
	}
	log.Printf("Conexão OK.")

	if err := export.WritePlaceholder(opt.Output, opt.Table, opt.Column, time.Now()); err != nil {
		log.Fatalf("erro ao gerar export: %v", err)
	}
	log.Printf("Arquivo %s criado (placeholder). Próximo passo: implementar coleta de relações e INSERTs.", opt.Output)
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func ask(r *bufio.Reader, msg string) string {
	fmt.Print(msg)
	s, _ := r.ReadString('\n')
	return strings.TrimSpace(s)
}
