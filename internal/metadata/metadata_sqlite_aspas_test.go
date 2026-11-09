package metadata

import (
	"context"
	"database/sql"
	"reflect"
	"testing"

	_ "modernc.org/sqlite"
)

// Issue #64: o catálogo do SQLite precisa achar colunas e chaves estrangeiras
// de tabelas cujo nome contém aspas (PRAGMA table_info e foreign_key_list).
func TestDiscoverSQLiteTabelaComAspasNoNome(t *testing.T) {
	casos := []struct {
		nome, pai, filha, coluna string
		ddl                      []string
	}{
		{
			nome: "aspas_duplas", pai: `p"q`, filha: `t"x`, coluna: `c"y`,
			ddl: []string{
				`CREATE TABLE "p""q" (id INTEGER PRIMARY KEY)`,
				`CREATE TABLE "t""x" (id INTEGER PRIMARY KEY, "c""y" TEXT, p_id INTEGER, FOREIGN KEY (p_id) REFERENCES "p""q"(id))`,
			},
		},
		{
			nome: "crases", pai: "p`q", filha: "t`x", coluna: "c`y",
			ddl: []string{
				"CREATE TABLE \"p`q\" (id INTEGER PRIMARY KEY)",
				"CREATE TABLE \"t`x\" (id INTEGER PRIMARY KEY, \"c`y\" TEXT, p_id INTEGER, FOREIGN KEY (p_id) REFERENCES \"p`q\"(id))",
			},
		},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			db, err := sql.Open("sqlite", "file:meta_"+c.nome+"?mode=memory&cache=shared")
			if err != nil {
				t.Fatalf("abrindo sqlite: %v", err)
			}
			t.Cleanup(func() { _ = db.Close() })
			for _, q := range c.ddl {
				if _, err := db.Exec(q); err != nil {
					t.Fatalf("executando %q: %v", q, err)
				}
			}

			catalog, err := Discover(context.Background(), db, "sqlite")
			if err != nil {
				t.Fatalf("descobrindo metadata sqlite: %v", err)
			}

			tabela, ok := catalog.Table(c.filha)
			if !ok {
				t.Fatalf("tabela %q não encontrada", c.filha)
			}
			achou := false
			for _, col := range tabela.Columns {
				if col.Name == c.coluna {
					achou = true
				}
			}
			if len(tabela.Columns) != 3 || !achou {
				t.Fatalf("colunas de %q inesperadas (esperava id, %q, p_id): %+v", c.filha, c.coluna, tabela.Columns)
			}
			if !tabela.Columns[0].PrimaryKey {
				t.Fatalf("id deveria ser a chave primária: %+v", tabela.Columns)
			}

			esperada := ForeignKey{Table: c.filha, Columns: []string{"p_id"}, RefTable: c.pai, RefColumns: []string{"id"}}
			achou = false
			for _, fk := range catalog.ForeignKeys {
				if reflect.DeepEqual(fk, esperada) {
					achou = true
				}
			}
			if !achou {
				t.Fatalf("chave estrangeira %+v não encontrada: %+v", esperada, catalog.ForeignKeys)
			}
		})
	}
}
