package metadata

import (
	"reflect"
	"sort"
	"strings"
	"testing"
)

// Os casos numerados seguem a seção "Casos de teste" de docs/RELACOES.md.

// tabelaT monta uma tabela no formato da doc: "col*" é chave primária.
// "col:TIPO" define o tipo da coluna.
func tabelaT(nome string, cols ...string) Table {
	t := Table{Name: nome}
	for _, spec := range cols {
		c := Column{}
		if strings.HasSuffix(spec, "*") {
			c.PrimaryKey = true
			spec = strings.TrimSuffix(spec, "*")
		}
		if i := strings.Index(spec, ":"); i >= 0 {
			c.Type = spec[i+1:]
			spec = spec[:i]
		}
		c.Name = spec
		t.Columns = append(t.Columns, c)
	}
	return t
}

func catalogoT(fks []ForeignKey, tabelas ...Table) Catalog {
	m := map[string]Table{}
	for _, t := range tabelas {
		m[strings.ToLower(t.Name)] = t
	}
	return Catalog{tables: m, ForeignKeys: fks}
}

func fkT(tabela, coluna, destino, colDestino string) ForeignKey {
	return ForeignKey{Table: tabela, Columns: []string{coluna}, RefTable: destino, RefColumns: []string{colDestino}}
}

func ordenaFKs(fks []ForeignKey) []ForeignKey {
	out := append([]ForeignKey(nil), fks...)
	sort.Slice(out, func(i, j int) bool { return lessForeignKey(out[i], out[j]) })
	return out
}

type casoInferencia struct {
	nome       string
	catalogo   Catalog
	declaradas []ForeignKey
	esperado   []ForeignKey // relações adicionadas
}

func rodaCasos(t *testing.T, casos []casoInferencia) {
	t.Helper()
	for _, c := range casos {
		c := c
		t.Run(c.nome, func(t *testing.T) {
			atualizado, n := c.catalogo.WithNameInferredForeignKeys()
			if n != len(c.esperado) {
				t.Fatalf("esperava %d relações adicionadas, obtive %d; lista: %+v", len(c.esperado), n, atualizado.ForeignKeys)
			}
			want := ordenaFKs(append(append([]ForeignKey(nil), c.declaradas...), c.esperado...))
			got := ordenaFKs(atualizado.ForeignKeys)
			if len(want) == 0 && len(got) == 0 {
				return
			}
			if !reflect.DeepEqual(want, got) {
				t.Fatalf("lista de relações diferente\nesperado: %+v\nobtido:   %+v", want, got)
			}
		})
	}
}

func TestInferenciaPorNomeCasosPositivos(t *testing.T) {
	rodaCasos(t, []casoInferencia{
		{nome: "01 plural com s",
			catalogo: catalogoT(nil, tabelaT("users", "id*"), tabelaT("orders", "id*", "user_id")),
			esperado: []ForeignKey{fkT("orders", "user_id", "users", "id")}},
		{nome: "02 singular exato",
			catalogo: catalogoT(nil, tabelaT("user", "id*"), tabelaT("orders", "id*", "user_id")),
			esperado: []ForeignKey{fkT("orders", "user_id", "user", "id")}},
		{nome: "03 plural em ies",
			catalogo: catalogoT(nil, tabelaT("categories", "id*"), tabelaT("products", "id*", "category_id")),
			esperado: []ForeignKey{fkT("products", "category_id", "categories", "id")}},
		{nome: "04 maiúsculas",
			catalogo: catalogoT(nil, tabelaT("Users", "id*"), tabelaT("Orders", "id*", "USER_ID")),
			esperado: []ForeignKey{fkT("Orders", "USER_ID", "Users", "id")}},
		{nome: "05 coluna no plural",
			catalogo: catalogoT(nil, tabelaT("users", "id*"), tabelaT("orders", "id*", "users_id")),
			esperado: []ForeignKey{fkT("orders", "users_id", "users", "id")}},
		{nome: "06 nome base com underscore",
			catalogo: catalogoT(nil, tabelaT("order_items", "id*"), tabelaT("shipments", "id*", "order_item_id")),
			esperado: []ForeignKey{fkT("shipments", "order_item_id", "order_items", "id")}},
		{nome: "07 auto-referência pelo nome da tabela",
			catalogo: catalogoT(nil, tabelaT("categories", "id*", "category_id")),
			esperado: []ForeignKey{fkT("categories", "category_id", "categories", "id")}},
		{nome: "08 várias colunas na mesma tabela",
			catalogo: catalogoT(nil, tabelaT("users", "id*"), tabelaT("posts", "id*", "user_id", "author_id"), tabelaT("authors", "id*")),
			esperado: []ForeignKey{fkT("posts", "user_id", "users", "id"), fkT("posts", "author_id", "authors", "id")}},
		{nome: "09 exato vence plural",
			catalogo: catalogoT(nil, tabelaT("user", "id*"), tabelaT("users", "id*"), tabelaT("orders", "id*", "user_id")),
			esperado: []ForeignKey{fkT("orders", "user_id", "user", "id")}},
		{nome: "10 chave primária composta com id",
			catalogo: catalogoT(nil, tabelaT("users", "id*", "tenant*"), tabelaT("orders", "id*", "user_id")),
			esperado: []ForeignKey{fkT("orders", "user_id", "users", "id")}},
		{nome: "11 tipos diferentes são aceitos",
			catalogo: catalogoT(nil, tabelaT("users", "id:INTEGER*"), tabelaT("orders", "id*", "user_id:TEXT")),
			esperado: []ForeignKey{fkT("orders", "user_id", "users", "id")}},
	})
}

func TestInferenciaPorNomeCasosNegativos(t *testing.T) {
	rodaCasos(t, []casoInferencia{
		{nome: "12 plural em es",
			catalogo: catalogoT(nil, tabelaT("addresses", "id*"), tabelaT("orders", "id*", "address_id"))},
		{nome: "13 plural irregular",
			catalogo: catalogoT(nil, tabelaT("people", "id*"), tabelaT("orders", "id*", "person_id"))},
		{nome: "14 prefixo na coluna",
			catalogo: catalogoT(nil, tabelaT("addresses", "id*"), tabelaT("orders", "id*", "billing_address_id"))},
		{nome: "15 prefixo na tabela",
			catalogo: catalogoT(nil, tabelaT("app_users", "id*"), tabelaT("orders", "id*", "user_id"))},
		{nome: "16 auto-referência com outro nome",
			catalogo: catalogoT(nil, tabelaT("categories", "id*", "parent_id"))},
		{nome: "17 singular terminado em s com tabela no plural",
			catalogo: catalogoT(nil, tabelaT("statuses", "id*"), tabelaT("orders", "id*", "status_id"))},
		{nome: "18 sufixo diferente",
			catalogo: catalogoT(nil, tabelaT("users", "id*"), tabelaT("orders", "id*", "user_code", "userid", "user_fk"))},
		{nome: "19 camelCase",
			catalogo: catalogoT(nil, tabelaT("users", "id*"), tabelaT("orders", "id*", "userId"))},
		{nome: "20 coluna id sozinha",
			catalogo: catalogoT(nil, tabelaT("users", "id*"), tabelaT("orders", "id*"))},
		{nome: "21 tabela inexistente",
			catalogo: catalogoT(nil, tabelaT("users", "id*"), tabelaT("orders", "id*", "coupon_id"))},
		{nome: "22 destino sem coluna id",
			catalogo: catalogoT(nil, tabelaT("users", "user_id*"), tabelaT("orders", "id*", "user_id"))},
		{nome: "23 destino com id fora da chave",
			catalogo: catalogoT(nil, tabelaT("users", "id", "name*"), tabelaT("orders", "id*", "user_id"))},
		{nome: "24 ambiguidade entre plurais",
			catalogo: catalogoT(nil, tabelaT("babys", "id*"), tabelaT("babies", "id*"), tabelaT("x", "id*", "baby_id"))},
		{nome: "25 coluna _id sozinha",
			catalogo: catalogoT(nil, tabelaT("users", "id*"), tabelaT("x", "id*", "_id"))},
		{nome: "26 FK composta não declarada",
			catalogo: catalogoT(nil, tabelaT("parent", "a*", "b*"), tabelaT("child", "id*", "a_id", "b_id"))},
	})
}

func TestInferenciaPorNomeCatalogoVazio(t *testing.T) { // caso 27
	vazio := catalogoT(nil)
	atualizado, n := vazio.WithNameInferredForeignKeys()
	if n != 0 {
		t.Fatalf("esperava 0 relações, obtive %d", n)
	}
	if len(atualizado.ForeignKeys) != 0 || len(atualizado.TableNames()) != 0 {
		t.Fatalf("esperava catálogo vazio, obtive %+v", atualizado)
	}
}

func TestInferenciaPorNomeComForeignKeysExistentes(t *testing.T) {
	realUsers := fkT("orders", "user_id", "users", "id")
	realCustomers := fkT("orders", "user_id", "customers", "id")
	comp := ForeignKey{Name: "fk_child_parent", Table: "child", Columns: []string{"a_id", "b_id"}, RefTable: "parent", RefColumns: []string{"a", "b"}}

	rodaCasos(t, []casoInferencia{
		{nome: "28 FK real tem prioridade",
			catalogo: catalogoT([]ForeignKey{realCustomers},
				tabelaT("users", "id*"), tabelaT("customers", "id*"), tabelaT("orders", "id*", "user_id")),
			declaradas: []ForeignKey{realCustomers}},
		{nome: "29 FK real igual à inferida",
			catalogo:   catalogoT([]ForeignKey{realUsers}, tabelaT("users", "id*"), tabelaT("orders", "id*", "user_id")),
			declaradas: []ForeignKey{realUsers}},
		{nome: "30 FK composta declarada",
			catalogo: catalogoT([]ForeignKey{comp},
				tabelaT("parent", "a*", "b*"), tabelaT("child", "id*", "a_id", "b_id")),
			declaradas: []ForeignKey{comp}},
	})
}

func TestInferenciaPorNomeRodarDuasVezes(t *testing.T) { // caso 31
	c := catalogoT(nil, tabelaT("users", "id*"), tabelaT("orders", "id*", "user_id"))
	primeiro, n1 := c.WithNameInferredForeignKeys()
	if n1 != 1 {
		t.Fatalf("primeira chamada: esperava 1, obtive %d", n1)
	}
	segundo, n2 := primeiro.WithNameInferredForeignKeys()
	if n2 != 0 {
		t.Fatalf("segunda chamada: esperava 0, obtive %d", n2)
	}
	if !reflect.DeepEqual(primeiro.ForeignKeys, segundo.ForeignKeys) {
		t.Fatalf("lista mudou na segunda chamada\nantes:  %+v\ndepois: %+v", primeiro.ForeignKeys, segundo.ForeignKeys)
	}
}

func TestInferenciaPorNomeOrdemDoResultado(t *testing.T) { // caso 32
	c := catalogoT(nil, tabelaT("users", "id*"), tabelaT("posts", "id*", "user_id", "author_id"), tabelaT("authors", "id*"))
	atualizado, n := c.WithNameInferredForeignKeys()
	if n != 2 {
		t.Fatalf("esperava 2 adicionadas, obtive %d", n)
	}
	want := []ForeignKey{fkT("posts", "author_id", "authors", "id"), fkT("posts", "user_id", "users", "id")}
	if !reflect.DeepEqual(atualizado.ForeignKeys, want) {
		t.Fatalf("ordem inesperada\nesperado: %+v\nobtido:   %+v", want, atualizado.ForeignKeys)
	}
}

func TestInferenciaPorNomeFalsosPositivos(t *testing.T) {
	rodaCasos(t, []casoInferencia{
		{nome: "33 id de outro sistema",
			catalogo: catalogoT(nil, tabelaT("users", "id*"), tabelaT("audit", "id*", "user_id")),
			esperado: []ForeignKey{fkT("audit", "user_id", "users", "id")}},
		{nome: "34 auto-relação não pretendida",
			catalogo: catalogoT(nil, tabelaT("employees", "id*", "employee_id")),
			esperado: []ForeignKey{fkT("employees", "employee_id", "employees", "id")}},
	})
}
