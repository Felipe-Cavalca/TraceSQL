package export

import (
	"context"
	"os"
	"testing"

	"github.com/Felipe-Cavalca/TraceSQL/internal/config"
)

// itCaso é uma linha da tabela da matriz: monta a origem, exporta, restaura no
// destino e compara. Cada caso roda em todas as combinações origem -> destino
// disponíveis (SQLite -> SQLite sem tag; 3x3 com a tag `integration`).
type itCaso struct {
	nome   string
	setup  func(t *testing.T, b *itBanco)
	tabela string
	coluna string // padrão "id"
	valor  string

	novosIDs     bool
	profundidade *int
	porNome      bool
	ignoraSufixo string

	esperado map[string]int // linhas esperadas no destino, por tabela
	semFKs   bool           // não compara FKs (relações inferidas por nome)
	origens  []string       // restringe as origens (nomes dos motores); vazio = todas

	// bugs: "origem>destino" ("*" vale para qualquer lado) -> motivo; o caso é
	// pulado com t.Skip("bug ...") e a expectativa NÃO muda para caber no código.
	bugs map[string]string

	// depois roda no destino já restaurado (checagens além da comparação).
	depois func(t *testing.T, ctx context.Context, dest *itBanco)
}

func itInt(n int) *int { return &n }

// itBugs associa o mesmo motivo a várias combinações ("origem>destino"; "*" vale
// para qualquer motor). Sem número de issue, a chave é TODO-<chave>: troque pelo
// número da issue quando ela existir.
func itBugs(motivo string, chaves ...string) map[string]string {
	m := map[string]string{}
	for _, c := range chaves {
		m[c] = motivo
	}
	return m
}

const (
	bugBoolMySQL = "bug TODO-bool-mysql-pg: tinyint(1) do MySQL sai como literal inteiro (0/1) e o Postgres rejeita em coluna boolean"
	bugFracao    = "bug TODO-fracao-segundo: timestamp(6)/datetime(6) vira DATETIME sem precisão no MySQL e a fração de segundo é perdida"
	bugOrdemFK   = "bug TODO-ordem-tabelas-auto-fk: uma tabela com FK para si mesma impede a ordenação topológica e as tabelas dependentes saem antes do pai"
	bugOrdemLin  = "bug TODO-ordem-linhas-auto-fk: sem --new-ids as linhas de uma tabela auto-referenciada saem ordenadas como texto (10 antes de 9) e o filho antecede o pai"
	bugCicloDDL  = "bug TODO-ciclo-fk: FK mútua gera CREATE TABLE com FK para tabela ainda inexistente (Postgres/MySQL) e INSERTs que violam a FK (SQLite)"
	bugCicloNovo = "bug TODO-ciclo-fk: FK mútua com --new-ids: DDL falha em Postgres/MySQL e, no SQLite, a referência adiante vira NULL sem erro"
	bugIdentPG   = "bug TODO-identidade-pg: a sequência de identidade do Postgres não avança após restaurar com ids originais (próximo INSERT sem id repete o id 1)"
	bugNumLivre  = "bug TODO-numeric-sem-precisao: numeric sem precisão vira DECIMAL(10,0) no MySQL e perde as casas decimais"
	bugIntNome   = "bug TODO-tipo-por-substring: interval e point viram INTEGER no DDL (mapColumnType usa strings.Contains(\"int\"))"
	bugUnsigned  = "bug TODO-unsigned: int/bigint unsigned estouram INTEGER/BIGINT no Postgres e no MySQL, e passam de int64 no SQLite (vira REAL)"
)

func itCasos() []itCaso {
	loja := map[string]int{"categorias": 2, "clientes": 2, "produtos": 2, "pedidos": 2, "itens": 3, "pedidos_log": 1}
	return []itCaso{
		{nome: "tipos", setup: itSetupTipos, tabela: "tipos_lote", valor: "1",
			esperado: map[string]int{"tipos_lote": 1, "tipos": 10}},
		{nome: "tipos_novos_ids", setup: itSetupTipos, tabela: "tipos_lote", valor: "1", novosIDs: true,
			esperado: map[string]int{"tipos_lote": 1, "tipos": 10}},
		{nome: "booleanos", setup: itSetupBooleanos, tabela: "tipos_lote", valor: "1", bugs: itBugs(bugBoolMySQL, "mysql>postgres"),
			esperado: map[string]int{"tipos_lote": 1, "booleanos": 3}},
		{nome: "fracao_de_segundo", setup: itSetupFracao, tabela: "fracao", valor: "1", bugs: itBugs(bugFracao, "*>mysql"),
			esperado: map[string]int{"fracao": 1}},
		{nome: "barra_invertida_63", setup: itSetupNotas, tabela: "cadernos", valor: "1",
			esperado: map[string]int{"cadernos": 1, "notas": 8}},
		{nome: "nomes_com_aspas_64", setup: itSetupEstranha, tabela: itNomeTabelaEstranho, valor: "1",
			esperado: map[string]int{itNomeTabelaEstranho: 1}},
		{nome: "loja_completa", setup: itSetupLoja, tabela: "pedidos", valor: "1001", esperado: loja},
		{nome: "loja_novos_ids", setup: itSetupLoja, tabela: "pedidos", valor: "1001", novosIDs: true, esperado: loja},
		{nome: "loja_profundidade_0", setup: itSetupLoja, tabela: "pedidos", valor: "1001", profundidade: itInt(0),
			esperado: map[string]int{"pedidos": 1}},
		{nome: "loja_profundidade_1", setup: itSetupLoja, tabela: "pedidos", valor: "1001", profundidade: itInt(1),
			esperado: map[string]int{"clientes": 1, "pedidos": 1, "itens": 2, "pedidos_log": 1}},
		{nome: "loja_ignora_sufixo_log", setup: itSetupLoja, tabela: "pedidos", valor: "1001", ignoraSufixo: "_log",
			esperado: map[string]int{"categorias": 2, "clientes": 2, "produtos": 2, "pedidos": 2, "itens": 3}},
		{nome: "loja_com_categorias_em_arvore", setup: itSetupLojaArvore, tabela: "pedidos", valor: "1001", bugs: itBugs(bugOrdemFK, "*>*"),
			esperado: map[string]int{"categorias": 3, "clientes": 2, "produtos": 2, "pedidos": 2, "itens": 3, "pedidos_log": 1}},
		{nome: "categorias_auto_referencia", setup: itSetupCategorias, tabela: "categorias", valor: "100", bugs: itBugs(bugOrdemLin, "*>*"),
			esperado: map[string]int{"categorias": 3}},
		{nome: "categorias_auto_referencia_novos_ids", setup: itSetupCategorias, tabela: "categorias", valor: "100", novosIDs: true,
			esperado: map[string]int{"categorias": 3}},
		{nome: "ciclo", setup: itSetupCiclo, tabela: "departamentos", valor: "1", bugs: itBugs(bugCicloDDL, "*>*"),
			esperado: map[string]int{"departamentos": 1, "funcionarios": 2}},
		{nome: "ciclo_novos_ids", setup: itSetupCiclo, tabela: "departamentos", valor: "1", novosIDs: true, bugs: itBugs(bugCicloNovo, "*>*"),
			esperado: map[string]int{"departamentos": 1, "funcionarios": 2}},
		{nome: "relacoes_por_nome", setup: itSetupRelacaoPorNome, tabela: "authors", valor: "1", porNome: true, semFKs: true,
			esperado: map[string]int{"authors": 1, "books": 2}},
		{nome: "autoincremento_apos_restaurar", setup: itSetupContas, tabela: "contas", valor: "1",
			esperado: map[string]int{"contas": 1}, depois: itInsereDepoisDeRestaurar, bugs: itBugs(bugIdentPG, "*>postgres")},
		{nome: "extras_postgres", setup: itSetupExtrasPG, tabela: "extras_pg", valor: "1", origens: []string{"postgres"}, bugs: itBugs(bugNumLivre, "postgres>mysql"),
			esperado: map[string]int{"extras_pg": 1}},
		{nome: "extras_postgres_nulos", setup: itSetupExtrasPG, tabela: "extras_pg", valor: "2", origens: []string{"postgres"},
			esperado: map[string]int{"extras_pg": 1}},
		{nome: "extras_postgres_intervalo_e_ponto", setup: itSetupExtrasPGIntervalo, tabela: "extras_pg_intervalo", valor: "1", origens: []string{"postgres"}, bugs: itBugs(bugIntNome, "postgres>postgres", "postgres>mysql"),
			esperado: map[string]int{"extras_pg_intervalo": 1}},
		{nome: "extras_mysql", setup: itSetupExtrasMySQL, tabela: "extras_mysql", valor: "1", origens: []string{"mysql"},
			esperado: map[string]int{"extras_mysql": 1}},
		{nome: "extras_mysql_nulos", setup: itSetupExtrasMySQL, tabela: "extras_mysql", valor: "3", origens: []string{"mysql"},
			esperado: map[string]int{"extras_mysql": 1}},
		{nome: "extras_mysql_sem_sinal", setup: itSetupExtrasMySQLSemSinal, tabela: "extras_mysql_sem_sinal", valor: "1", origens: []string{"mysql"}, bugs: itBugs(bugUnsigned, "*>*"),
			esperado: map[string]int{"extras_mysql_sem_sinal": 1}},
		{nome: "extras_sqlite", setup: itSetupExtrasSQLite, tabela: "extras_sqlite", valor: "1", origens: []string{"sqlite"},
			esperado: map[string]int{"extras_sqlite": 1}},
	}
}

// itInsereDepoisDeRestaurar confere que a tabela restaurada com os ids originais
// ainda aceita uma linha nova sem informar o id (a sequência avançou).
func itInsereDepoisDeRestaurar(t *testing.T, ctx context.Context, dest *itBanco) {
	t.Helper()
	_, err := dest.db.ExecContext(ctx, "INSERT INTO "+itQuote(dest.motor, "contas")+" ("+itQuote(dest.motor, "codigo")+") VALUES ('conta-nova')")
	if err != nil {
		t.Errorf("inserir sem id após restaurar com ids originais: %v", err)
	}
}

func TestIntegracaoMatriz(t *testing.T) {
	motores := itMotores()
	for _, c := range itCasos() {
		for _, orig := range motores {
			if len(c.origens) > 0 && !itContem(c.origens, orig.nome) {
				continue
			}
			for _, dest := range motores {
				t.Run(c.nome+"/"+orig.nome+"_para_"+dest.nome, func(t *testing.T) {
					// TRACESQL_TEST_RUN_KNOWN_BUGS=1 roda também os casos marcados como bug
					// conhecido (para ver se ainda falham ou se já foram corrigidos).
					if os.Getenv("TRACESQL_TEST_RUN_KNOWN_BUGS") == "" {
						for _, chave := range []string{orig.nome + ">" + dest.nome, "*>" + dest.nome, orig.nome + ">*", "*>*"} {
							if motivo, ok := c.bugs[chave]; ok {
								t.Skip(motivo)
							}
						}
					}
					itExecutaCaso(t, c, orig, dest)
				})
			}
		}
	}
}

func itContem(lista []string, s string) bool {
	for _, v := range lista {
		if v == s {
			return true
		}
	}
	return false
}

func itExecutaCaso(t *testing.T, c itCaso, orig, dest itMotor) {
	t.Helper()
	ctx := itContexto(t)

	bo := itNovoBanco(t, orig)
	bd := itNovoBanco(t, dest)
	c.setup(t, bo)

	col := c.coluna
	if col == "" {
		col = "id"
	}
	cfg := config.Config{
		Driver:            orig.driver,
		OutputDriver:      dest.driver,
		DSN:               "unused",
		Table:             c.tabela,
		Column:            col,
		Record:            c.valor,
		NewIDs:            c.novosIDs,
		RelationsByName:   c.porNome,
		Depth:             c.profundidade,
		IgnoreTableSuffix: c.ignoraSufixo,
	}
	dump, err := Run(ctx, bo.db, cfg)
	if err != nil {
		t.Fatalf("exportar: %v", err)
	}
	if err := itRestaura(ctx, bd, dump); err != nil {
		t.Fatalf("restaurar o .sql no destino: %v\n--- SQL exportado ---\n%s", err, dump)
	}
	itCompara(t, ctx, bo, bd, c.esperado, c.novosIDs, c.semFKs)
	if c.depois != nil {
		c.depois(t, ctx, bd)
	}
	if t.Failed() {
		t.Logf("--- SQL exportado ---\n%s", dump)
	}
}
