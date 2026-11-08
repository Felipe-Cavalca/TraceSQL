package export

import (
	"context"
	"strings"
	"testing"

	"github.com/Felipe-Cavalca/TraceSQL/internal/config"
)

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
