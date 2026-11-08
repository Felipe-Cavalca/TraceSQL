package export

import (
	"context"
	"strings"
	"testing"

	"github.com/Felipe-Cavalca/TraceSQL/internal/config"
)

// Referência com --new-ids: o valor antigo da chave (texto com barra e aspa)
// usa o mesmo literal do dialeto de saída (docs/EXPORTACAO.md, "Referência com --new-ids").
func TestRunExportNewIDsReferenciaComBarraEAspa(t *testing.T) {
	casos := []struct {
		dialeto string
		want    string
	}{
		{"mysql", "WHERE `old_id` = " + `'a\\b''c')`},
		{"postgres", `WHERE "old_id" = 'a\b''c')`},
		{"sqlite", "WHERE `old_id` = " + `'a\b''c')`},
	}
	for _, c := range casos {
		t.Run(c.dialeto, func(t *testing.T) {
			db := openSQLiteTestDB(t)
			mustExec(t, db, "CREATE TABLE users (code TEXT PRIMARY KEY, name TEXT)")
			mustExec(t, db, "CREATE TABLE orders (id INTEGER PRIMARY KEY, user_code TEXT, total INT, FOREIGN KEY(user_code) REFERENCES users(code))")
			mustExec(t, db, `INSERT INTO users (code, name) VALUES ('a\b''c', 'Alice')`)
			mustExec(t, db, `INSERT INTO orders (id, user_code, total) VALUES (10, 'a\b''c', 99)`)

			cfg := config.Config{Driver: "sqlite", OutputDriver: c.dialeto, DSN: "unused", Table: "users", Column: "code", Record: `a\b'c`, NewIDs: true}
			dump, err := Run(context.Background(), db, cfg)
			if err != nil {
				t.Fatalf("erro ao exportar: %v", err)
			}
			if !strings.Contains(dump, c.want) {
				t.Fatalf("referência esperada com %s\nsaída:\n%s", c.want, dump)
			}
		})
	}
}

// Ida e volta ponta a ponta no mysql: o INSERT gerado por Run, lido pela regra
// do mysql, devolve o texto original.
func TestRunExportMySQLIdaEVoltaComBarraEAspa(t *testing.T) {
	for _, v := range []string{`C:\temp\'x`, `\'; DROP TABLE x;--`, "l1\r\nl2", "a\x1ab"} {
		t.Run(escNomeValor(v), func(t *testing.T) {
			db := openSQLiteTestDB(t)
			mustExec(t, db, "CREATE TABLE notas (id INTEGER PRIMARY KEY, texto TEXT)")
			if _, err := db.Exec("INSERT INTO notas (id, texto) VALUES (1, ?)", v); err != nil {
				t.Fatalf("inserir: %v", err)
			}
			cfg := config.Config{Driver: "sqlite", OutputDriver: "mysql", DSN: "unused", Table: "notas", Column: "id", Record: "1"}
			dump, err := Run(context.Background(), db, cfg)
			if err != nil {
				t.Fatalf("erro ao exportar: %v", err)
			}
			const ini = "VALUES (1, "
			i := strings.Index(dump, ini)
			if i < 0 {
				t.Fatalf("INSERT não encontrado:\n%s", dump)
			}
			resto := dump[i+len(ini):]
			j := strings.LastIndex(resto[:strings.Index(resto, ");\n")+1], ")")
			if j < 0 {
				t.Fatalf("fim do INSERT não encontrado:\n%s", dump)
			}
			got, err := escDecodificaMySQL(resto[:j])
			if err != nil {
				t.Fatalf("literal inválido %q: %v", resto[:j], err)
			}
			if got != v {
				t.Fatalf("ida e volta: lido %q, quer %q", got, v)
			}
		})
	}
}
