package metadata

import (
	"context"
	"database/sql"
	"reflect"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	_ "modernc.org/sqlite"
)

func TestDiscoverSQLiteForeignKeyCompostaEDuasFKsComColunasCompartilhadas(t *testing.T) {
	db, err := sql.Open("sqlite", "file:metadata_fk_composta?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	for _, q := range []string{
		"CREATE TABLE parent (a INTEGER NOT NULL, b INTEGER NOT NULL, PRIMARY KEY (a, b))",
		"CREATE TABLE other (a INTEGER NOT NULL, c INTEGER NOT NULL, PRIMARY KEY (a, c))",
		// Colunas declaradas fora da ordem da referência, para provar a ordem posicional.
		"CREATE TABLE child (id INTEGER PRIMARY KEY, a INTEGER, b INTEGER, c INTEGER, " +
			"FOREIGN KEY (b, a) REFERENCES parent (b, a), FOREIGN KEY (a, c) REFERENCES other (a, c))",
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatalf("%q: %v", q, err)
		}
	}

	catalog, err := Discover(context.Background(), db, "sqlite")
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}

	want := []ForeignKey{
		{Table: "child", Columns: []string{"a", "c"}, RefTable: "other", RefColumns: []string{"a", "c"}},
		{Table: "child", Columns: []string{"b", "a"}, RefTable: "parent", RefColumns: []string{"b", "a"}},
	}
	if !reflect.DeepEqual(catalog.ForeignKeys, want) {
		t.Fatalf("FKs inesperadas:\n got %+v\nwant %+v", catalog.ForeignKeys, want)
	}
	if !catalog.ForeignKeys[0].IsComposite() {
		t.Fatal("esperava FK composta")
	}
}

func TestDiscoverPostgresAgrupaFKCompostaPorConstraint(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	mock.ExpectQuery(`SELECT tablename`).WillReturnRows(sqlmock.NewRows([]string{"tablename"}).AddRow("child"))
	mock.ExpectQuery(`SELECT\s+a\.attname`).WithArgs("child").
		WillReturnRows(sqlmock.NewRows([]string{"attname", "format_type", "is_nullable", "default_value", "is_primary_key", "is_auto_increment"}).
			AddRow("a", "integer", true, nil, false, false))
	mock.ExpectQuery(`FROM pg_catalog\.pg_constraint`).WithArgs("child").
		WillReturnRows(sqlmock.NewRows([]string{"conname", "relname", "attname", "ref_table", "ref_column"}).
			AddRow("fk_other", "child", "a", "other", "a").
			AddRow("fk_other", "child", "c", "other", "c").
			AddRow("fk_parent", "child", "a", "parent", "a").
			AddRow("fk_parent", "child", "b", "parent", "b"))

	catalog, err := Discover(context.Background(), db, "postgres")
	if err != nil {
		t.Fatal(err)
	}
	want := []ForeignKey{
		{Name: "fk_parent", Table: "child", Columns: []string{"a", "b"}, RefTable: "parent", RefColumns: []string{"a", "b"}},
		{Name: "fk_other", Table: "child", Columns: []string{"a", "c"}, RefTable: "other", RefColumns: []string{"a", "c"}},
	}
	if !reflect.DeepEqual(catalog.ForeignKeys, want) {
		t.Fatalf("FKs inesperadas:\n got %+v\nwant %+v", catalog.ForeignKeys, want)
	}
}

func TestDiscoverMySQLAgrupaFKCompostaPorConstraint(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	mock.ExpectQuery(`FROM information_schema\.tables`).WillReturnRows(sqlmock.NewRows([]string{"table_name"}).AddRow("child"))
	mock.ExpectQuery(`FROM information_schema\.columns`).WithArgs("child").
		WillReturnRows(sqlmock.NewRows([]string{"column_name", "column_type", "is_nullable", "column_key", "extra"}).
			AddRow("a", "int", "YES", "", ""))
	mock.ExpectQuery(`FROM information_schema\.key_column_usage`).WithArgs("child").
		WillReturnRows(sqlmock.NewRows([]string{"constraint_name", "table_name", "column_name", "referenced_table_name", "referenced_column_name"}).
			AddRow("fk_other", "child", "a", "other", "a").
			AddRow("fk_other", "child", "c", "other", "c").
			AddRow("fk_parent", "child", "a", "parent", "a").
			AddRow("fk_parent", "child", "b", "parent", "b"))

	catalog, err := Discover(context.Background(), db, "mysql")
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog.ForeignKeys) != 2 || !reflect.DeepEqual(catalog.ForeignKeys[0].Columns, []string{"a", "b"}) ||
		!reflect.DeepEqual(catalog.ForeignKeys[1].RefColumns, []string{"a", "c"}) {
		t.Fatalf("FKs inesperadas: %+v", catalog.ForeignKeys)
	}
}

func TestInferenciaPorNomeNaoInfereCompostaECompostaBloqueiaColunas(t *testing.T) {
	comp := ForeignKey{Table: "child", Columns: []string{"user_id", "b"}, RefTable: "parent", RefColumns: []string{"a", "b"}}
	c := catalogoT([]ForeignKey{comp},
		tabelaT("users", "id*"), tabelaT("parent", "a*", "b*"), tabelaT("child", "id*", "user_id", "b"))
	_, n := c.WithNameInferredForeignKeys()
	if n != 0 {
		t.Fatalf("coluna que já participa de FK composta não deveria ser inferida, obtive %d", n)
	}
}
