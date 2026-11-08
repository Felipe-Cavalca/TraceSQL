package export

import (
	"context"
	"database/sql"
	"strings"
	"testing"

	"github.com/Felipe-Cavalca/TraceSQL/internal/config"
)

// Testes de docs/IDENTIFICADORES.md (issue #64): quoteIdent deve escapar o
// caractere de aspas do dialeto dentro do nome, e não removê-lo.

const crase = "`"

type qiCaso struct {
	nome    string
	driver  string
	entrada string
	saida   string
}

// qiDesfazer desfaz o escape como a doc descreve: tira o primeiro e o último
// caractere e troca cada par do caractere de aspas por um só.
func qiDesfazer(driver, quoted string) (string, bool) {
	q := `"`
	if strings.ToLower(driver) == "mysql" {
		q = crase
	}
	if len(quoted) < 2 || !strings.HasPrefix(quoted, q) || !strings.HasSuffix(quoted, q) {
		return "", false
	}
	miolo := quoted[1 : len(quoted)-1]
	return strings.ReplaceAll(miolo, q+q, q), true
}

func qiRodar(t *testing.T, casos []qiCaso, desfazer bool) {
	t.Helper()
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			got := quoteIdent(c.driver, c.entrada)
			if got != c.saida {
				t.Fatalf("quoteIdent(%q, %q) = %q, esperado %q", c.driver, c.entrada, got, c.saida)
			}
			if !desfazer {
				return
			}
			volta, ok := qiDesfazer(c.driver, got)
			if !ok {
				t.Fatalf("saída %q não está envolvida pelo caractere de aspas", got)
			}
			if volta != c.entrada {
				t.Fatalf("desfazer o escape de %q deu %q, esperado %q (nome alterado)", got, volta, c.entrada)
			}
		})
	}
}

func qiCasosEscape() []qiCaso {
	return []qiCaso{
		{"simples_postgres", "postgres", "users", `"users"`},
		{"simples_mysql", "mysql", "users", crase + "users" + crase},
		{"simples_sqlite", "sqlite", "users", `"users"`},
		{"aspa_dupla_postgres", "postgres", `a"b`, `"a""b"`},
		{"aspa_dupla_sqlite", "sqlite", `a"b`, `"a""b"`},
		{"aspa_dupla_mysql_preservada", "mysql", `a"b`, crase + `a"b` + crase},
		{"crase_mysql", "mysql", "a`b", "`a``b`"},
		{"crase_postgres_preservada", "postgres", "a`b", "\"a`b\""},
		{"crase_sqlite_preservada", "sqlite", "a`b", "\"a`b\""},
		{"varias_aspas_postgres", "postgres", `a"b"c`, `"a""b""c"`},
		{"varias_crases_mysql", "mysql", "a`b`c", "`a``b``c`"},
		{"so_aspa_postgres", "postgres", `"`, `""""`},
		{"so_crase_mysql", "mysql", "`", "````"},
		{"aspas_seguidas_postgres", "postgres", `""`, `""""""`},
		{"aspa_no_inicio_e_fim_postgres", "postgres", `"x"`, `"""x"""`},
		{"crase_no_inicio_e_fim_mysql", "mysql", "`x`", "```x```"},
		{"os_dois_tipos_postgres", "postgres", "a\"b`c", "\"a\"\"b`c\""},
		{"os_dois_tipos_mysql", "mysql", "a\"b`c", "`a\"b``c`"},
		{"os_dois_tipos_sqlite", "sqlite", "a\"b`c", "\"a\"\"b`c\""},
	}
}

func TestQuoteIdentEscapePorDialeto(t *testing.T) {
	qiRodar(t, qiCasosEscape(), true)
}

func TestQuoteIdentNomeDoDialeto(t *testing.T) {
	qiRodar(t, []qiCaso{
		{"dialeto_pg", "pg", `a"b`, `"a""b"`},
		{"dialeto_postgresql", "postgresql", `a"b`, `"a""b"`},
		{"dialeto_postgres_maiusculo", "POSTGRES", `a"b`, `"a""b"`},
		{"dialeto_sqlite3", "sqlite3", `a"b`, `"a""b"`},
		{"dialeto_mysql_maiusculo", "MySQL", "a`b", "`a``b`"},
		{"dialeto_desconhecido", "oracle", `a"b`, `"a""b"`},
		{"dialeto_desconhecido_crase_preservada", "oracle", "a`b", "\"a`b\""},
		{"dialeto_vazio", "", `a"b`, `"a""b"`},
		{"dialeto_vazio_nome_simples", "", "users", `"users"`},
	}, true)
}

func TestQuoteIdentNomesSemAspasNaoMudam(t *testing.T) {
	entradas := []struct{ nome, entrada string }{
		{"espaco", "my table"},
		{"espacos_nas_pontas", " x "},
		{"ponto_nao_separa", "public.users"},
		{"unicode", "usuário_ção"},
		{"emoji", "tab😀"},
		{"apostrofo", "o'brien"},
		{"ponto_e_virgula", "a;b"},
		{"comentario_sql", "a--b"},
		{"barra_invertida", `a\b`},
		{"caixa_preservada", "UserID"},
	}
	for _, d := range []struct{ driver, q string }{
		{"postgres", `"`},
		{"mysql", crase},
		{"sqlite", `"`},
	} {
		var casos []qiCaso
		for _, e := range entradas {
			casos = append(casos, qiCaso{e.nome + "_" + d.driver, d.driver, e.entrada, d.q + e.entrada + d.q})
		}
		qiRodar(t, casos, true)
	}
}

func TestQuoteIdentNomeVazioDevolveVazio(t *testing.T) {
	qiRodar(t, []qiCaso{
		{"vazio_postgres", "postgres", "", ""},
		{"vazio_mysql", "mysql", "", ""},
		{"vazio_sqlite", "sqlite", "", ""},
		{"vazio_dialeto_vazio", "", "", ""},
	}, false)
}

func TestQuoteIdentNaoPanicaNemAlteraBytesInvalidos(t *testing.T) {
	qiRodar(t, []qiCaso{
		{"utf8_invalido_postgres", "postgres", "a\xffb", "\"a\xffb\""},
		{"byte_nulo_mysql", "mysql", "a\x00b", "`a\x00b`"},
		{"quebra_de_linha_sqlite", "sqlite", "a\nb", "\"a\nb\""},
	}, false)
}

// Integração: o banco entende o nome escapado (doc, "Integração").
func TestQuoteIdentIntegracaoSQLite(t *testing.T) {
	casos := []struct {
		nome, tabela, coluna string
		wantTabela           string
		wantColuna           string
	}{
		{"aspas_duplas", `t"x`, `c"y`, `"t""x"`, `"c""y"`},
		{"crases", "t`x", "c`y", "\"t`x\"", "\"c`y\""},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			tabela := quoteIdent("sqlite", c.tabela)
			coluna := quoteIdent("sqlite", c.coluna)
			if tabela != c.wantTabela || coluna != c.wantColuna {
				t.Fatalf("quoteIdent: tabela %q (esperado %q), coluna %q (esperado %q)", tabela, c.wantTabela, coluna, c.wantColuna)
			}

			db := openSQLiteTestDB(t)
			mustExec(t, db, "CREATE TABLE "+tabela+" ("+coluna+" TEXT)")
			mustExec(t, db, "INSERT INTO "+tabela+" ("+coluna+") VALUES ('valor')")

			var valor string
			if err := db.QueryRow("SELECT " + coluna + " FROM " + tabela).Scan(&valor); err != nil {
				t.Fatalf("lendo a linha de volta: %v", err)
			}
			if valor != "valor" {
				t.Fatalf("valor lido = %q, esperado %q", valor, "valor")
			}

			var n int
			if err := db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?", c.tabela).Scan(&n); err != nil {
				t.Fatalf("consultando sqlite_master: %v", err)
			}
			if n != 1 {
				t.Fatalf("sqlite_master deveria ter a tabela %q exatamente uma vez, achou %d", c.tabela, n)
			}

			var m int
			if err := db.QueryRow("SELECT COUNT(*) FROM pragma_table_info(?) WHERE name = ?", c.tabela, c.coluna).Scan(&m); err != nil {
				t.Fatalf("consultando pragma table_info: %v", err)
			}
			if m != 1 {
				t.Fatalf("pragma table_info(%q) deveria listar a coluna %q, achou %d", c.tabela, c.coluna, m)
			}
		})
	}
}

func abrirBancoDestino(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", "file:destino_"+strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())+"?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("abrindo sqlite de destino: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

// Exportação ponta a ponta com aspas nos nomes de tabela, coluna e chave estrangeira.
func TestRunExportNomesComAspasDuplasParaSQLite(t *testing.T) {
	db := openSQLiteTestDB(t)
	mustExec(t, db, `CREATE TABLE "p""q" (id INTEGER PRIMARY KEY, nome TEXT)`)
	mustExec(t, db, `CREATE TABLE "t""x" (id INTEGER PRIMARY KEY, "c""y" TEXT, p_id INTEGER, FOREIGN KEY (p_id) REFERENCES "p""q"(id))`)
	mustExec(t, db, `INSERT INTO "p""q" (id, nome) VALUES (7, 'pai')`)
	mustExec(t, db, `INSERT INTO "t""x" (id, "c""y", p_id) VALUES (1, 'valor', 7)`)

	cfg := config.Config{Driver: "sqlite", OutputDriver: "sqlite", DSN: "unused", Table: `t"x`, Column: "id", Record: "1"}
	dump, err := Run(context.Background(), db, cfg)
	if err != nil {
		t.Fatalf("erro ao exportar: %v", err)
	}

	containsAll(t, dump, `"t""x"`, `"c""y"`, `"p""q"`)
	for _, ruim := range []string{`"tx"`, `"cy"`, `"pq"`} {
		if strings.Contains(dump, ruim) {
			t.Fatalf("o dump não deveria conter %s (aspas removidas do nome):\n%s", ruim, dump)
		}
	}

	destino := abrirBancoDestino(t)
	if _, err := destino.Exec(dump); err != nil {
		t.Fatalf("executando o .sql num banco novo: %v\n%s", err, dump)
	}
	var n int
	if err := destino.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?", `t"x`).Scan(&n); err != nil {
		t.Fatalf("consultando sqlite_master: %v", err)
	}
	if n != 1 {
		t.Fatalf("a tabela t\"x deveria ser recriada com o nome original, achou %d\n%s", n, dump)
	}
	var valor string
	if err := destino.QueryRow(`SELECT "c""y" FROM "t""x" WHERE id = 1`).Scan(&valor); err != nil {
		t.Fatalf("lendo o registro restaurado: %v", err)
	}
	if valor != "valor" {
		t.Fatalf("valor restaurado = %q, esperado %q", valor, "valor")
	}
}

// A consulta SELECT * FROM <tabela> WHERE <coluna> = ? também escapa os nomes (doc, "Onde a regra vale").
func TestRunExportBuscaPorColunaComAspasNoNome(t *testing.T) {
	db := openSQLiteTestDB(t)
	mustExec(t, db, `CREATE TABLE "t""x" (id INTEGER PRIMARY KEY, "c""y" TEXT)`)
	mustExec(t, db, `INSERT INTO "t""x" (id, "c""y") VALUES (1, 'alvo'), (2, 'outro')`)

	cfg := config.Config{Driver: "sqlite", OutputDriver: "postgres", DSN: "unused", Table: `t"x`, Column: `c"y`, Record: "alvo"}
	dump, err := Run(context.Background(), db, cfg)
	if err != nil {
		t.Fatalf("erro ao exportar buscando pela coluna c\"y: %v", err)
	}
	containsAll(t, dump, `CREATE TABLE IF NOT EXISTS "t""x"`, `INSERT INTO "t""x" ("id", "c""y") VALUES (1, 'alvo');`)
	if strings.Contains(dump, "'outro'") {
		t.Fatalf("o dump não deveria incluir o registro 'outro':\n%s", dump)
	}
}

func TestRunExportNomesComCraseParaMySQLEPostgres(t *testing.T) {
	db := openSQLiteTestDB(t)
	mustExec(t, db, "CREATE TABLE \"t`x\" (id INTEGER PRIMARY KEY, \"c`y\" TEXT)")
	mustExec(t, db, "INSERT INTO \"t`x\" (id, \"c`y\") VALUES (1, 'v')")

	casos := []struct {
		dialeto string
		want    []string
	}{
		{"mysql", []string{"CREATE TABLE IF NOT EXISTS `t``x`", "INSERT INTO `t``x` (`id`, `c``y`) VALUES (1, 'v');"}},
		{"postgres", []string{"CREATE TABLE IF NOT EXISTS \"t`x\"", "INSERT INTO \"t`x\" (\"id\", \"c`y\") VALUES (1, 'v');"}},
		{"sqlite", []string{"CREATE TABLE IF NOT EXISTS \"t`x\"", "INSERT INTO \"t`x\" (\"id\", \"c`y\") VALUES (1, 'v');"}},
	}
	for _, c := range casos {
		t.Run(c.dialeto, func(t *testing.T) {
			cfg := config.Config{Driver: "sqlite", OutputDriver: c.dialeto, DSN: "unused", Table: "t`x", Column: "id", Record: "1"}
			dump, err := Run(context.Background(), db, cfg)
			if err != nil {
				t.Fatalf("erro ao exportar: %v", err)
			}
			containsAll(t, dump, c.want...)
		})
	}
}

func TestRunExportNomesComAspasDuplasParaMySQLMantemAspas(t *testing.T) {
	db := openSQLiteTestDB(t)
	mustExec(t, db, `CREATE TABLE "t""x" (id INTEGER PRIMARY KEY, "c""y" TEXT)`)
	mustExec(t, db, `INSERT INTO "t""x" (id, "c""y") VALUES (1, 'v')`)

	cfg := config.Config{Driver: "sqlite", OutputDriver: "mysql", DSN: "unused", Table: `t"x`, Column: "id", Record: "1"}
	dump, err := Run(context.Background(), db, cfg)
	if err != nil {
		t.Fatalf("erro ao exportar: %v", err)
	}
	containsAll(t, dump, "CREATE TABLE IF NOT EXISTS `t\"x`", "INSERT INTO `t\"x` (`id`, `c\"y`) VALUES (1, 'v');")
}

// Nome da tabela de mapa do --new-ids: aspas e crase do nome da tabela são
// preservadas (e escapadas ao escrever), os demais caracteres especiais viram "_".
// A doc não define esse formato (lacuna-doc).
func TestRunExportNewIDsNomeDaTabelaDeMapa(t *testing.T) {
	casos := []struct {
		nome, tabela, ddl, mapa string
	}{
		{"aspas_duplas", `t"x`, `CREATE TABLE "t""x" (id INTEGER PRIMARY KEY, nome TEXT)`, `"__tracesql_map_t""x"`},
		{"crase", "t`x", "CREATE TABLE \"t`x\" (id INTEGER PRIMARY KEY, nome TEXT)", "\"__tracesql_map_t`x\""},
		{"espaco", "my table", `CREATE TABLE "my table" (id INTEGER PRIMARY KEY, nome TEXT)`, `"__tracesql_map_my_table"`},
		{"ponto", "a.b", `CREATE TABLE "a.b" (id INTEGER PRIMARY KEY, nome TEXT)`, `"__tracesql_map_a_b"`},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			db := openSQLiteTestDB(t)
			mustExec(t, db, c.ddl)
			mustExec(t, db, "INSERT INTO "+quoteIdent("sqlite", c.tabela)+" (id, nome) VALUES (5, 'a')")

			cfg := config.Config{Driver: "sqlite", OutputDriver: "sqlite", DSN: "unused", Table: c.tabela, Column: "id", Record: "5", NewIDs: true}
			dump, err := Run(context.Background(), db, cfg)
			if err != nil {
				t.Fatalf("erro ao exportar com new_ids: %v", err)
			}
			containsAll(t, dump,
				"CREATE TEMP TABLE "+c.mapa,
				"INSERT INTO "+c.mapa+` ("old_id", "new_id") VALUES (5, last_insert_rowid());`,
			)
		})
	}
}

// Tabelas de mapeamento do --new-ids também escapam o nome (doc, "Onde a regra vale").
func TestRunExportNewIDsComAspasNoNomeDaTabela(t *testing.T) {
	db := openSQLiteTestDB(t)
	mustExec(t, db, `CREATE TABLE "t""x" (id INTEGER PRIMARY KEY, nome TEXT)`)
	mustExec(t, db, `INSERT INTO "t""x" (id, nome) VALUES (5, 'a')`)

	cfg := config.Config{Driver: "sqlite", OutputDriver: "sqlite", DSN: "unused", Table: `t"x`, Column: "id", Record: "5", NewIDs: true}
	dump, err := Run(context.Background(), db, cfg)
	if err != nil {
		t.Fatalf("erro ao exportar com new_ids: %v", err)
	}
	containsAll(t, dump,
		`CREATE TEMP TABLE "__tracesql_map_t""x"`,
		`INSERT INTO "t""x" ("nome") VALUES ('a');`,
		`INSERT INTO "__tracesql_map_t""x" ("old_id", "new_id") VALUES (5, last_insert_rowid());`,
	)
}
