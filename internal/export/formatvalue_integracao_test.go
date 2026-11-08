package export

import (
	"context"
	"strings"
	"testing"

	"github.com/Felipe-Cavalca/TraceSQL/internal/config"
)

// Casos de borda que a doc cobre pela regra geral (lista fechada e formato
// exato de data), sem linha própria nas tabelas.
func TestFormatValueBordasDaListaFechadaEDoFormatoDeData(t *testing.T) {
	fvRodarClasses(t, []fvClasse{
		{"array_colchetes_com_tamanho_nao_e_array", "integer[3]", categoryText},
		{"int_colchetes_com_tamanho", "int[3]", categoryText},
		{"tipo_com_sufixo_desconhecido", "int_big", categoryText},
	})

	fvRodar(t, []fvCaso{
		{"array_colchetes_com_tamanho_continua_texto", "postgres", "integer[3]", fvVal("{1,2,3}"), "'{1,2,3}'"},
		{"data_t_sem_segundos_mysql", "mysql", "timestamp", fvVal("2024-05-01T10:00Z"), "'2024-05-01T10:00Z'"},
		{"data_t_sem_segundos_sqlite", "sqlite", "datetime", fvVal("2024-05-01T10:00Z"), "'2024-05-01T10:00Z'"},
		{"data_t_so_horas_mysql", "mysql", "timestamp", fvVal("2024-05-01T10"), "'2024-05-01T10'"},
		{"data_fora_do_formato_mysql", "mysql", "datetime", fvVal("ontem T agora"), "'ontem T agora'"},
		{"data_com_aspas_fora_do_formato", "sqlite", "date", fvVal("a'b"), "'a''b'"},
	})
}

// Teste de integração da doc: exporta de um SQLite uma tabela com colunas dos
// tipos que antes saíam inválidos (INTERVAL, BLOB, JSON, BOOLEAN) e confere o
// INSERT gerado em cada dialeto de saída.
func TestRunExportLiteraisPorCategoriaEDialeto(t *testing.T) {
	casos := []struct {
		dialeto string
		want    string
	}{
		{"postgres", `VALUES (1, TRUE, '\x4869', 'it''s', '2024-05-01T10:00:00Z', '1 day', '{"a":1}');`},
		{"mysql", `VALUES (1, TRUE, X'4869', 'it''s', '2024-05-01 10:00:00', '1 day', '{"a":1}');`},
		{"sqlite", `VALUES (1, 1, X'4869', 'it''s', '2024-05-01 10:00:00', '1 day', '{"a":1}');`},
	}
	for _, c := range casos {
		t.Run(c.dialeto, func(t *testing.T) {
			db := openSQLiteTestDB(t)
			mustExec(t, db, "CREATE TABLE eventos (id INTEGER PRIMARY KEY, ativo BOOLEAN, anexo BLOB, nota TEXT, criado TIMESTAMP, duracao INTERVAL, meta JSON)")
			mustExec(t, db, `INSERT INTO eventos (id, ativo, anexo, nota, criado, duracao, meta) VALUES (1, 1, X'4869', 'it''s', '2024-05-01T10:00:00Z', '1 day', '{"a":1}')`)

			cfg := config.Config{
				Driver:       "sqlite",
				OutputDriver: c.dialeto,
				DSN:          "unused",
				Table:        "eventos",
				Column:       "id",
				Record:       "1",
			}
			dump, err := Run(context.Background(), db, cfg)
			if err != nil {
				t.Fatalf("erro ao exportar: %v", err)
			}
			if !strings.Contains(dump, c.want) {
				t.Fatalf("INSERT esperado com %s\nsaída:\n%s", c.want, dump)
			}
		})
	}
}

func TestRunExportValorNaoNumericoEmColunaComNomeContendoInt(t *testing.T) {
	db := openSQLiteTestDB(t)
	mustExec(t, db, "CREATE TABLE lugares (id INTEGER PRIMARY KEY, ponto POINT, nada INTERVAL)")
	mustExec(t, db, `INSERT INTO lugares (id, ponto, nada) VALUES (1, '(1,2)', NULL)`)

	cfg := config.Config{Driver: "sqlite", OutputDriver: "postgres", DSN: "unused", Table: "lugares", Column: "id", Record: "1"}
	dump, err := Run(context.Background(), db, cfg)
	if err != nil {
		t.Fatalf("erro ao exportar: %v", err)
	}
	if !strings.Contains(dump, `VALUES (1, '(1,2)', NULL);`) {
		t.Fatalf("ponto deveria sair entre aspas e nulo como NULL:\n%s", dump)
	}
}

// Exceção booleana do tinyint(1): só vale para o nome inteiro (docs/EXPORTACAO.md, Passo 1 e Decisão 10).
func TestTinyint1UnsignedEhNumerico(t *testing.T) {
	fvRodarClasses(t, []fvClasse{
		{"tinyint_1_unsigned_maiusculo", "TINYINT(1) UNSIGNED", categoryNumeric},
		{"tinyint_1_unsigned_minusculo", "tinyint(1) unsigned", categoryNumeric},
		{"tinyint_1_bool_para_comparacao", "TINYINT(1)", categoryBool},
		{"tinyint_1_aparado", "  tinyint(1) ", categoryBool},
	})

	var casos []fvCaso
	for _, tipo := range []string{"tinyint(1) unsigned", "TINYINT(1) UNSIGNED"} {
		casos = append(casos,
			fvCaso{"um/" + tipo, "", tipo, fvVal("1"), "1"},
			fvCaso{"zero/" + tipo, "", tipo, fvVal("0"), "0"},
		)
	}
	fvRodar(t, fvEmTodos(casos))

	fvRodar(t, []fvCaso{
		{"bool_postgres", "postgres", "TINYINT(1)", fvVal("1"), "TRUE"},
		{"bool_mysql", "mysql", "TINYINT(1)", fvVal("0"), "FALSE"},
		{"bool_sqlite", "sqlite", "TINYINT(1)", fvVal("1"), "1"},
	})
}

func TestFormatValueBordasDeDataEDialetoVazioDaDoc(t *testing.T) {
	fvRodar(t, []fvCaso{
		{"data_t_sem_segundos_mysql", "mysql", "datetime", fvVal("2024-05-01T10:00"), "'2024-05-01T10:00'"},
		{"data_fora_do_formato_sqlite", "sqlite", "timestamp", fvVal("2024-05-01T10:00:00 Z"), "'2024-05-01T10:00:00 Z'"},
		{"dialeto_vazio_usa_sqlite", "", "bool", fvVal("true"), "1"},
	})
}
