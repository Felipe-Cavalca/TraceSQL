package export

import (
	"context"
	"database/sql"
	"reflect"
	"strings"
	"testing"

	"github.com/Felipe-Cavalca/TraceSQL/internal/config"
	"github.com/Felipe-Cavalca/TraceSQL/internal/metadata"
)

// Cenário: child tem duas FKs distintas que compartilham a coluna a:
// (a, b) -> parent(a, b) e (a, c) -> other(a, c).
func criaCenarioFKComposta(t *testing.T) (*sql.DB, config.Config) {
	t.Helper()
	db := openSQLiteTestDB(t)
	mustExec(t, db, "PRAGMA foreign_keys = ON")
	mustExec(t, db, "CREATE TABLE parent (a INTEGER NOT NULL, b INTEGER NOT NULL, nome TEXT, PRIMARY KEY (a, b))")
	mustExec(t, db, "CREATE TABLE other (a INTEGER NOT NULL, c INTEGER NOT NULL, nome TEXT, PRIMARY KEY (a, c))")
	mustExec(t, db, "CREATE TABLE child (id INTEGER PRIMARY KEY, a INTEGER, b INTEGER, c INTEGER, "+
		"FOREIGN KEY (a, b) REFERENCES parent (a, b), FOREIGN KEY (a, c) REFERENCES other (a, c))")
	mustExec(t, db, "INSERT INTO parent (a, b, nome) VALUES (1, 1, 'p11'), (1, 2, 'p12'), (2, 1, 'p21')")
	mustExec(t, db, "INSERT INTO other (a, c, nome) VALUES (1, 5, 'o15'), (1, 6, 'o16')")
	mustExec(t, db, "INSERT INTO child (id, a, b, c) VALUES (10, 1, 2, 5), (11, 1, 1, 6)")
	return db, config.Config{Driver: "sqlite", OutputDriver: "sqlite", DSN: "unused", Table: "child", Column: "id", Record: "10"}
}

func TestRunExportFKCompostaUsaTodasAsColunasEPreservaRelacao(t *testing.T) {
	db, cfg := criaCenarioFKComposta(t)

	dump, err := Run(context.Background(), db, cfg)
	if err != nil {
		t.Fatalf("erro ao exportar: %v", err)
	}

	containsAll(t, dump,
		`FOREIGN KEY ("a", "b") REFERENCES "parent" ("a", "b")`,
		`FOREIGN KEY ("a", "c") REFERENCES "other" ("a", "c")`,
		`'p12'`,
		`'o15'`,
	)
	for _, indevido := range []string{`'p11'`, `'p21'`, `'o16'`, "(11, "} {
		if strings.Contains(dump, indevido) {
			t.Fatalf("o dump não deveria conter %s (colunas de FKs diferentes misturadas): %s", indevido, dump)
		}
	}

	// Restaura em um banco novo e confere que a relação foi preservada.
	restored, err := sql.Open("sqlite", "file:fk_composta_restaurado?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = restored.Close() })
	mustExec(t, restored, "PRAGMA foreign_keys = ON")
	mustExec(t, restored, dump)

	catalog, err := metadata.Discover(context.Background(), restored, "sqlite")
	if err != nil {
		t.Fatalf("descobrindo metadata restaurada: %v", err)
	}
	var got [][]string
	for _, fk := range catalog.ForeignKeys {
		got = append(got, append(append([]string{fk.Table, fk.RefTable}, fk.Columns...), fk.RefColumns...))
	}
	want := [][]string{
		{"child", "parent", "a", "b", "a", "b"},
		{"child", "other", "a", "c", "a", "c"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("FKs restauradas inesperadas: %v (esperava %v)", got, want)
	}
}

func TestRunExportFKCompostaPaiAteFilhos(t *testing.T) {
	db, _ := criaCenarioFKComposta(t)

	// A partir do pai (1, 2) só o filho 10 é alcançado, não o 11.
	cfg := config.Config{Driver: "sqlite", OutputDriver: "sqlite", DSN: "unused", Table: "parent", Column: "b", Record: "2"}
	dump, err := Run(context.Background(), db, cfg)
	if err != nil {
		t.Fatalf("erro ao exportar: %v", err)
	}
	containsAll(t, dump, `'p12'`, `'o15'`)
	// parent.b = 2 só casa com (1, 2); o filho 11 (a=1, b=1) não pode entrar.
	if strings.Contains(dump, `'p11'`) || strings.Contains(dump, `'o16'`) || strings.Contains(dump, "(11, ") {
		t.Fatalf("o dump trouxe linhas fora da relação composta: %s", dump)
	}
}

func TestBuildCreateTableForeignKeyComposta(t *testing.T) {
	table := metadata.Table{
		Name: "child",
		Columns: []metadata.Column{
			{Name: "id", Type: "integer", PrimaryKey: true},
			{Name: "a", Type: "integer", Nullable: true},
			{Name: "b", Type: "integer", Nullable: true},
		},
	}
	fks := []metadata.ForeignKey{{Table: "child", Columns: []string{"a", "b"}, RefTable: "parent", RefColumns: []string{"x", "y"}}}

	casos := map[string]string{
		"postgres": `FOREIGN KEY ("a", "b") REFERENCES "parent" ("x", "y")`,
		"mysql":    "FOREIGN KEY (`a`, `b`) REFERENCES `parent` (`x`, `y`)",
		"sqlite":   `FOREIGN KEY ("a", "b") REFERENCES "parent" ("x", "y")`,
	}
	for driver, esperado := range casos {
		stmt := buildCreateTable(driver, table, fks, nil)
		if !strings.Contains(stmt, esperado) {
			t.Errorf("%s: esperava %q em:\n%s", driver, esperado, stmt)
		}
	}
}

func TestBuildCreateTableForeignKeySimplesInalterada(t *testing.T) {
	table := metadata.Table{Name: "orders", Columns: []metadata.Column{{Name: "id", Type: "integer", PrimaryKey: true}, {Name: "user_id", Type: "integer", Nullable: true}}}
	fks := []metadata.ForeignKey{{Table: "orders", Columns: []string{"user_id"}, RefTable: "users", RefColumns: []string{"id"}}}
	if stmt := buildCreateTable("postgres", table, fks, nil); !strings.Contains(stmt, `FOREIGN KEY ("user_id") REFERENCES "users" ("id")`) {
		t.Fatalf("FK simples mudou: %s", stmt)
	}
}
