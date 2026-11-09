package export

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	mysqldrv "github.com/go-sql-driver/mysql"

	"github.com/Felipe-Cavalca/TraceSQL/internal/db"
)

// itMotor é um motor de banco que pode ser origem ou destino da matriz.
type itMotor struct {
	nome   string // nome usado nos subtestes
	driver string // valor de --driver / --output-driver
	envDSN string // variável com o DSN de administração ("" para o SQLite)
}

var (
	itSQLite   = itMotor{"sqlite", "sqlite", ""}
	itPostgres = itMotor{"postgres", "postgres", "TRACESQL_TEST_PG_DSN"}
	itMySQL    = itMotor{"mysql", "mysql", "TRACESQL_TEST_MYSQL_DSN"}
)

// idx devolve 0 (postgres), 1 (mysql) ou 2 (sqlite): índice dos tipos nativos da fixture.
func (m itMotor) idx() int {
	switch m.driver {
	case "postgres":
		return 0
	case "mysql":
		return 1
	}
	return 2
}

// itBanco é um banco criado só para um teste.
type itBanco struct {
	motor itMotor
	nome  string
	db    *sql.DB
}

var itContador atomic.Int64

func itContexto(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	t.Cleanup(cancel)
	return ctx
}

// itNovoBanco cria um banco novo e vazio no motor e o apaga ao final do teste.
// Sem DSN (Postgres/MySQL), pula o teste.
func itNovoBanco(t *testing.T, m itMotor) *itBanco {
	t.Helper()
	ctx := itContexto(t)
	b := &itBanco{motor: m}

	if m.driver == "sqlite" {
		conn, err := db.Open("sqlite", ":memory:")
		if err != nil {
			t.Fatalf("abrir sqlite: %v", err)
		}
		// Uma conexão só: cada conexão de ":memory:" seria um banco diferente.
		conn.SetMaxOpenConns(1)
		if _, err := conn.ExecContext(ctx, "PRAGMA foreign_keys = ON"); err != nil {
			t.Fatalf("PRAGMA foreign_keys: %v", err)
		}
		t.Cleanup(func() { _ = conn.Close() })
		b.nome = ":memory:"
		b.db = conn
		return b
	}

	raw := os.Getenv(m.envDSN)
	if raw == "" {
		t.Skipf("%s não definida", m.envDSN)
	}
	b.nome = fmt.Sprintf("tsql_it_%d_%d", os.Getpid(), itContador.Add(1))

	switch m.driver {
	case "postgres":
		admin, err := db.Open("postgres", raw)
		if err != nil {
			t.Fatalf("abrir admin do postgres: %v", err)
		}
		if _, err := admin.ExecContext(ctx, `CREATE DATABASE "`+b.nome+`"`); err != nil {
			_ = admin.Close()
			t.Fatalf("CREATE DATABASE: %v", err)
		}
		u, err := url.Parse(raw)
		if err != nil {
			t.Fatalf("DSN do postgres inválida: %v", err)
		}
		u.Path = "/" + b.nome
		conn, err := db.Open("postgres", u.String())
		if err != nil {
			t.Fatalf("abrir banco do postgres: %v", err)
		}
		b.db = conn
		t.Cleanup(func() {
			_ = conn.Close()
			_, _ = admin.Exec(`DROP DATABASE IF EXISTS "` + b.nome + `" WITH (FORCE)`)
			_ = admin.Close()
		})
	case "mysql":
		cfg, err := mysqldrv.ParseDSN(raw)
		if err != nil {
			t.Fatalf("DSN do mysql inválida: %v", err)
		}
		admin, err := db.Open("mysql", cfg.FormatDSN())
		if err != nil {
			t.Fatalf("abrir admin do mysql: %v", err)
		}
		if _, err := admin.ExecContext(ctx, "CREATE DATABASE `"+b.nome+"` CHARACTER SET utf8mb4"); err != nil {
			_ = admin.Close()
			t.Fatalf("CREATE DATABASE: %v", err)
		}
		cfg.DBName = b.nome
		cfg.MultiStatements = true
		if cfg.Params == nil {
			cfg.Params = map[string]string{}
		}
		cfg.Params["charset"] = "utf8mb4"
		conn, err := db.Open("mysql", cfg.FormatDSN())
		if err != nil {
			t.Fatalf("abrir banco do mysql: %v", err)
		}
		b.db = conn
		t.Cleanup(func() {
			_ = conn.Close()
			_, _ = admin.Exec("DROP DATABASE IF EXISTS `" + b.nome + "`")
			_ = admin.Close()
		})
	}
	return b
}

// itRebind troca ? por $n no Postgres.
func itRebind(m itMotor, q string) string {
	if m.driver != "postgres" {
		return q
	}
	var sb strings.Builder
	n := 0
	for _, r := range q {
		if r == '?' {
			n++
			fmt.Fprintf(&sb, "$%d", n)
			continue
		}
		sb.WriteRune(r)
	}
	return sb.String()
}

// itQuote protege um identificador conforme o dialeto (dobra o próprio delimitador).
func itQuote(m itMotor, ident string) string {
	if m.driver == "mysql" {
		return "`" + strings.ReplaceAll(ident, "`", "``") + "`"
	}
	return `"` + strings.ReplaceAll(ident, `"`, `""`) + `"`
}

func (b *itBanco) exec(t *testing.T, q string, args ...any) {
	t.Helper()
	if _, err := b.db.ExecContext(itContexto(t), itRebind(b.motor, q), args...); err != nil {
		t.Fatalf("[%s] falha ao executar %q: %v", b.motor.nome, q, err)
	}
}

// itRestaura executa o .sql exportado inteiro numa única conexão (as tabelas
// temporárias do --new-ids só existem na conexão que as criou).
func itRestaura(ctx context.Context, b *itBanco, script string) error {
	conn, err := b.db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	_, err = conn.ExecContext(ctx, script)
	return err
}
