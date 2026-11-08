// Package integration restaura, em bancos reais, o SQL que o TraceSQL exporta
// e compara o resultado com a origem. Veja docs/TESTES.md.
package integration

import (
	"context"
	"crypto/rand"
	"crypto/sha1"
	"database/sql"
	"encoding/hex"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	mysqldrv "github.com/go-sql-driver/mysql"

	"github.com/Felipe-Cavalca/TraceSQL/internal/db"
)

// engine é um motor de banco usado como origem ou destino.
type engine struct {
	label  string // nome usado nos subtestes: pg, mysql, sqlite, mariadb
	driver string // valor de --driver / --output-driver: postgres, mysql, sqlite
	envDSN string // variável com o DSN de administração ("" para o SQLite)
}

var (
	enginePG      = engine{"pg", "postgres", "TRACESQL_IT_POSTGRES_DSN"}
	engineMySQL   = engine{"mysql", "mysql", "TRACESQL_IT_MYSQL_DSN"}
	engineSQLite  = engine{"sqlite", "sqlite", ""}
	engineMariaDB = engine{"mariadb", "mysql", "TRACESQL_IT_MARIADB_DSN"}

	allEngines = []engine{enginePG, engineMySQL, engineSQLite}
)

// idx devolve 0 (postgres), 1 (mysql ou mariadb) ou 2 (sqlite).
func (e engine) idx() int {
	switch e.driver {
	case "postgres":
		return 0
	case "mysql":
		return 1
	}
	return 2
}

// require pula (ou falha, com TRACESQL_IT_REQUIRE=1) quando o motor não está disponível.
func require(t *testing.T, e engine) {
	t.Helper()
	if e.envDSN == "" || os.Getenv(e.envDSN) != "" {
		return
	}
	if e.label != "mariadb" && os.Getenv("TRACESQL_IT_REQUIRE") == "1" {
		t.Fatalf("%s não definido e TRACESQL_IT_REQUIRE=1", e.envDSN)
	}
	t.Skipf("%s não definido", e.envDSN)
}

// knownBug marca o subteste como bug conhecido, ainda sem issue.
// Troque o texto "a abrir" pelo número da issue quando ela existir.
func knownBug(t *testing.T, resumo string) {
	t.Helper()
	t.Skip("bug a abrir: " + resumo)
}

// testDB é um banco criado só para um teste.
type testDB struct {
	eng  engine
	name string
	role string
	db   *sql.DB
}

var runID = func() string {
	b := make([]byte, 3)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}()

func shortHash(s string) string {
	h := sha1.Sum([]byte(s))
	return hex.EncodeToString(h[:4])
}

func ctxTimeout(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	t.Cleanup(cancel)
	return ctx
}

// mysqlDSN acrescenta multiStatements e charset ao DSN de administração.
func mysqlDSN(t *testing.T, raw, dbName string) string {
	t.Helper()
	cfg, err := mysqldrv.ParseDSN(raw)
	if err != nil {
		t.Fatalf("DSN do MySQL inválido: %v", err)
	}
	cfg.MultiStatements = true
	cfg.ParseTime = false
	if cfg.Params == nil {
		cfg.Params = map[string]string{}
	}
	if _, ok := cfg.Params["charset"]; !ok {
		cfg.Params["charset"] = "utf8mb4"
	}
	if dbName != "" {
		cfg.DBName = dbName
	}
	return cfg.FormatDSN()
}

// pgDSN troca o nome do banco e garante timezone=UTC.
func pgDSN(t *testing.T, raw, dbName string) string {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("DSN do Postgres inválido: %v", err)
	}
	if dbName != "" {
		u.Path = "/" + dbName
	}
	q := u.Query()
	if q.Get("timezone") == "" {
		q.Set("timezone", "UTC")
	}
	u.RawQuery = q.Encode()
	return u.String()
}

// newDB cria um banco novo e vazio no motor e o apaga ao final do teste.
func newDB(t *testing.T, e engine, role string) *testDB {
	t.Helper()
	require(t, e)
	ctx := ctxTimeout(t)
	name := fmt.Sprintf("it_%s_%s_%s", shortHash(t.Name()), runID, role)
	d := &testDB{eng: e, name: name, role: role}

	switch e.driver {
	case "postgres":
		admin, err := db.Open("postgres", pgDSN(t, os.Getenv(e.envDSN), ""))
		if err != nil {
			t.Fatalf("abrir admin do Postgres: %v", err)
		}
		if _, err := admin.ExecContext(ctx, `CREATE DATABASE "`+name+`"`); err != nil {
			_ = admin.Close()
			t.Fatalf("CREATE DATABASE: %v", err)
		}
		conn, err := db.Open("postgres", pgDSN(t, os.Getenv(e.envDSN), name))
		if err != nil {
			t.Fatalf("abrir banco do Postgres: %v", err)
		}
		d.db = conn
		t.Cleanup(func() {
			_ = conn.Close()
			_, _ = admin.Exec(`DROP DATABASE IF EXISTS "` + name + `" WITH (FORCE)`)
			_ = admin.Close()
		})
	case "mysql":
		admin, err := db.Open("mysql", mysqlDSN(t, os.Getenv(e.envDSN), ""))
		if err != nil {
			t.Fatalf("abrir admin do MySQL: %v", err)
		}
		if _, err := admin.ExecContext(ctx, "CREATE DATABASE `"+name+"` CHARACTER SET utf8mb4"); err != nil {
			_ = admin.Close()
			t.Fatalf("CREATE DATABASE: %v", err)
		}
		conn, err := db.Open("mysql", mysqlDSN(t, os.Getenv(e.envDSN), name))
		if err != nil {
			t.Fatalf("abrir banco do MySQL: %v", err)
		}
		d.db = conn
		t.Cleanup(func() {
			_ = conn.Close()
			_, _ = admin.Exec("DROP DATABASE IF EXISTS `" + name + "`")
			_ = admin.Close()
		})
	default:
		dsn := "file:" + name + "?mode=memory&cache=shared"
		if role == "dst" {
			dsn += "&_pragma=foreign_keys(1)"
		}
		conn, err := db.Open("sqlite", dsn)
		if err != nil {
			t.Fatalf("abrir SQLite em memória: %v", err)
		}
		// Uma conexão fica aberta até o fim para o banco em memória não sumir.
		keeper, err := conn.Conn(ctx)
		if err != nil {
			t.Fatalf("conexão do SQLite: %v", err)
		}
		d.db = conn
		t.Cleanup(func() {
			_ = keeper.Close()
			_ = conn.Close()
		})
	}
	return d
}

// rebind troca ? por $n no Postgres.
func (d *testDB) rebind(q string) string {
	if d.eng.driver != "postgres" {
		return q
	}
	var b strings.Builder
	n := 0
	for _, r := range q {
		if r == '?' {
			n++
			fmt.Fprintf(&b, "$%d", n)
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// mustExec roda um comando da fixture e falha o teste se der erro.
func (d *testDB) mustExec(t *testing.T, q string, args ...any) {
	t.Helper()
	if _, err := d.db.ExecContext(ctxTimeout(t), d.rebind(q), args...); err != nil {
		t.Fatalf("[%s/%s] %v\nSQL: %s", d.eng.label, d.role, err, q)
	}
}

// quote escapa um identificador no dialeto do banco.
func (d *testDB) quote(name string) string { return quoteFor(d.eng, name) }

func quoteFor(e engine, name string) string {
	if e.driver == "mysql" {
		return "`" + strings.ReplaceAll(name, "`", "``") + "`"
	}
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

// matrixCombos devolve as combinações origem/destino disponíveis para a matriz de 9
// (mais as do MariaDB, que só rodam com TRACESQL_IT_MARIADB_DSN).
func matrixCombos() [][2]engine {
	var out [][2]engine
	for _, s := range allEngines {
		for _, d := range allEngines {
			out = append(out, [2]engine{s, d})
		}
	}
	for _, s := range allEngines {
		for _, d := range allEngines {
			if s.label != "mysql" && d.label != "mysql" {
				continue
			}
			ms, md := s, d
			if s.label == "mysql" {
				ms = engineMariaDB
			}
			if d.label == "mysql" {
				md = engineMariaDB
			}
			out = append(out, [2]engine{ms, md})
		}
	}
	return out
}

func comboName(s, d engine) string { return s.label + "_para_" + d.label }

// forMatrix roda fn nas 9 combinações (e nas do MariaDB, se configurado).
func forMatrix(t *testing.T, fn func(t *testing.T, src, dst engine)) {
	t.Helper()
	for _, c := range matrixCombos() {
		src, dst := c[0], c[1]
		t.Run(comboName(src, dst), func(t *testing.T) {
			require(t, src)
			require(t, dst)
			fn(t, src, dst)
		})
	}
}

// forDiagonal roda fn só em pg_para_pg, mysql_para_mysql e sqlite_para_sqlite.
func forDiagonal(t *testing.T, fn func(t *testing.T, e engine)) {
	t.Helper()
	for _, e := range allEngines {
		t.Run(comboName(e, e), func(t *testing.T) {
			require(t, e)
			fn(t, e)
		})
	}
}

// forSource roda fn com a origem fixa e os 3 destinos.
func forSource(t *testing.T, src engine, fn func(t *testing.T, src, dst engine)) {
	t.Helper()
	for _, dst := range allEngines {
		t.Run(comboName(src, dst), func(t *testing.T) {
			require(t, src)
			require(t, dst)
			fn(t, src, dst)
		})
	}
}
