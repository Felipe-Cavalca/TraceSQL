package integration

import (
	"strings"
	"testing"
)

// colDef descreve uma coluna da fixture, com o tipo nativo de cada motor
// (postgres, mysql, sqlite).
type colDef struct {
	name    string
	kind    string // int, dec, float, bool, text, bin, json, data, hora, ts
	typ     [3]string
	notNull bool
	unique  bool
	pk      bool
	fk      [2]string // tabela e coluna referenciadas ("" se não há)
}

func ct(pg, my, lite string) [3]string { return [3]string{pg, my, lite} }

var (
	tBig    = ct("bigint", "bigint", "INTEGER")
	tBigCol = ct("bigint", "bigint", "BIGINT")
	tInt    = ct("integer", "int", "INTEGER")
	tCod    = ct("varchar(20)", "varchar(20)", "VARCHAR(20)")
	tV40    = ct("varchar(40)", "varchar(40)", "VARCHAR(40)")
	tTs     = ct("timestamp", "datetime", "TIMESTAMP")
	tDec102 = ct("numeric(10,2)", "decimal(10,2)", "NUMERIC(10,2)")
)

var pgCast = map[string]string{
	"dec": "numeric", "json": "jsonb", "data": "date", "hora": "time", "ts": "timestamp",
}

// createTable monta o CREATE TABLE no dialeto do banco. Com withFK falso, as
// chaves estrangeiras ficam de fora (para serem adicionadas depois).
func createTable(t *testing.T, d *testDB, table string, cols []colDef, withFK bool) {
	t.Helper()
	i := d.eng.idx()
	var parts []string
	for _, c := range cols {
		p := d.quote(c.name) + " " + c.typ[i]
		if c.pk {
			p += " PRIMARY KEY"
		}
		if c.notNull {
			p += " NOT NULL"
		}
		if c.unique {
			p += " UNIQUE"
		}
		parts = append(parts, p)
	}
	if withFK {
		for _, c := range cols {
			if c.fk[0] != "" {
				parts = append(parts, "FOREIGN KEY ("+d.quote(c.name)+") REFERENCES "+d.quote(c.fk[0])+" ("+d.quote(c.fk[1])+")")
			}
		}
	}
	d.mustExec(t, "CREATE TABLE "+d.quote(table)+" ("+strings.Join(parts, ", ")+")")
}

// addFKs adiciona as chaves estrangeiras com ALTER TABLE (Postgres e MySQL).
func addFKs(t *testing.T, d *testDB, table string, cols []colDef) {
	t.Helper()
	for _, c := range cols {
		if c.fk[0] != "" {
			d.mustExec(t, "ALTER TABLE "+d.quote(table)+" ADD FOREIGN KEY ("+d.quote(c.name)+") REFERENCES "+d.quote(c.fk[0])+" ("+d.quote(c.fk[1])+")")
		}
	}
}

// insert grava uma linha. vals mapeia coluna para valor Go (nil é NULL; bool,
// int, float64, string e []byte). Colunas fora do mapa ficam NULL.
func insert(t *testing.T, d *testDB, table string, cols []colDef, vals map[string]any) {
	t.Helper()
	var names, ph []string
	var args []any
	for _, c := range cols {
		v, ok := vals[c.name]
		if !ok {
			continue
		}
		names = append(names, d.quote(c.name))
		p := "?"
		if d.eng.idx() == 0 {
			if cast, ok := pgCast[c.kind]; ok {
				p = "CAST(CAST(? AS text) AS " + cast + ")"
			}
		}
		ph = append(ph, p)
		if b, isBool := v.(bool); isBool && d.eng.idx() != 0 {
			if b {
				v = 1
			} else {
				v = 0
			}
		}
		args = append(args, v)
	}
	d.mustExec(t, "INSERT INTO "+d.quote(table)+" ("+strings.Join(names, ", ")+") VALUES ("+strings.Join(ph, ", ")+")", args...)
}

// ---------------------------------------------------------------- tipos

var tiposCols = []colDef{
	{name: "id", kind: "int", typ: ct("bigint", "bigint", "INTEGER"), pk: true},
	{name: "codigo", kind: "text", typ: ct("varchar(20)", "varchar(20)", "VARCHAR(20)"), notNull: true, unique: true},
	{name: "lote", kind: "text", typ: ct("varchar(10)", "varchar(10)", "VARCHAR(10)"), notNull: true},
	{name: "c_int", kind: "int", typ: ct("integer", "int", "INTEGER")},
	{name: "c_big", kind: "int", typ: ct("bigint", "bigint", "BIGINT")},
	{name: "c_dec", kind: "dec", typ: ct("numeric(12,2)", "decimal(12,2)", "NUMERIC(12,2)")},
	{name: "c_float", kind: "float", typ: ct("double precision", "double", "REAL")},
	{name: "c_bool", kind: "bool", typ: ct("boolean", "tinyint(1)", "BOOLEAN")},
	{name: "c_char", kind: "text", typ: ct("varchar(40)", "varchar(40)", "VARCHAR(40)")},
	{name: "c_texto", kind: "text", typ: ct("text", "longtext", "TEXT")},
	{name: "c_bin", kind: "bin", typ: ct("bytea", "longblob", "BLOB")},
	{name: "c_json", kind: "json", typ: ct("jsonb", "json", "JSON")},
	{name: "c_data", kind: "data", typ: ct("date", "date", "DATE")},
	{name: "c_hora", kind: "hora", typ: ct("time", "time", "TIME")},
	{name: "c_ts", kind: "ts", typ: ct("timestamp", "datetime", "TIMESTAMP")},
	{name: "c_ts_frac", kind: "ts", typ: ct("timestamp(6)", "datetime(6)", "TIMESTAMP")},
}

func tiposRows() []map[string]any {
	return []map[string]any{
		{"id": 101, "codigo": "r01-normal", "lote": "A", "c_int": 42, "c_big": int64(9000000000), "c_dec": "99.90", "c_float": 1.5,
			"c_bool": true, "c_char": "abc", "c_texto": "olá mundo", "c_bin": []byte{0x48, 0x69},
			"c_json": `{"a":1,"b":[true,null,"x"]}`, "c_data": "2024-05-01", "c_hora": "10:30:00",
			"c_ts": "2024-05-01 10:00:00", "c_ts_frac": "2024-05-01 10:00:00.123456"},
		{"id": 102, "codigo": "r02-extremos", "lote": "A", "c_int": -2147483648, "c_big": int64(9223372036854775807), "c_dec": "9999999999.99",
			"c_float": -0.25, "c_bool": false, "c_char": strings.Repeat("x", 40), "c_texto": strings.Repeat("z", 10000),
			"c_bin": []byte{0x00, 0xFF, 0x27, 0x5C, 0x5C, 0x00}, "c_json": `[]`, "c_data": "1970-01-01", "c_hora": "00:00:00",
			"c_ts": "1970-01-01 00:00:00", "c_ts_frac": "1999-12-31 23:59:59.999999"},
		{"id": 103, "codigo": "r03-barras-aspas", "lote": "A", "c_texto": `C:\temp\'x`, "c_char": `it's "q"`,
			"c_json": `{"p":"C:\\temp","q":"it's"}`},
		{"id": 104, "codigo": "r04-injecao", "lote": "A", "c_texto": `'; DROP TABLE tipos;--`, "c_char": `\'; DROP TABLE tipos;--`},
		{"id": 105, "codigo": "r05-unicode", "lote": "A", "c_texto": "ação ñ 日本語 😀 العربية", "c_char": "cafe\u0301"},
		{"id": 106, "codigo": "r06-controle", "lote": "A", "c_texto": "l1\nl2\r\nl3\ttab\x1asub"},
		{"id": 107, "codigo": "r07-vazios", "lote": "A", "c_texto": "", "c_char": "", "c_bin": []byte{}, "c_json": `null`},
		{"id": 108, "codigo": "r08-todos-nulos", "lote": "A"},
		{"id": 109, "codigo": "r09-barra-n-escrito", "lote": "A", "c_texto": `\n`, "c_char": `\\`, "c_json": `{"nl":"a\nb"}`},
		{"id": 110, "codigo": "r10-zeros", "lote": "A", "c_int": 0, "c_dec": "0.00", "c_float": 0.0, "c_ts": "2000-02-29 12:00:00"},
	}
}

func setupTipos(t *testing.T, d *testDB) {
	t.Helper()
	createTable(t, d, "tipos", tiposCols, true)
	for _, r := range tiposRows() {
		r["id"] = int64(r["id"].(int))
		insert(t, d, "tipos", tiposCols, r)
	}
	if d.eng.idx() == 2 {
		// O driver do SQLite grava blob vazio como NULL; o literal garante o vazio.
		d.mustExec(t, "UPDATE tipos SET c_bin = X'' WHERE id = 107")
	}
}

// ---------------------------------------------------------------- extras

func setupExtrasPG(t *testing.T, d *testDB) {
	t.Helper()
	d.mustExec(t, `CREATE TABLE extras_pg (id bigint PRIMARY KEY, codigo varchar(20) NOT NULL UNIQUE, lote varchar(10) NOT NULL,
		c_interval interval, c_uuid uuid, c_array integer[], c_ponto point, c_tstz timestamptz, c_numlivre numeric, c_bpchar character(5))`)
	d.mustExec(t, `INSERT INTO extras_pg VALUES (1, 'ex-1', 'A', '1 day 02:03:04', '3f2b8c3e-0000-4000-8000-000000000000',
		'{1,2,3}', '(1,2)', '2024-05-01 10:00:00+02', 3.14159, 'ab')`)
	d.mustExec(t, `INSERT INTO extras_pg (id, codigo, lote) VALUES (2, 'ex-nulos', 'A')`)
}

func setupExtrasMySQL(t *testing.T, d *testDB) {
	t.Helper()
	d.mustExec(t, `CREATE TABLE extras_mysql (id bigint PRIMARY KEY, codigo varchar(20) NOT NULL UNIQUE, lote varchar(10) NOT NULL,
		c_flag tinyint(1), c_ano year, c_enum enum('verde','azul'))`)
	d.mustExec(t, `INSERT INTO extras_mysql VALUES (1, 'em-1', 'A', 1, 2024, 'azul')`)
	d.mustExec(t, `INSERT INTO extras_mysql (id, codigo, lote, c_flag) VALUES (2, 'em-0', 'A', 0)`)
	d.mustExec(t, `INSERT INTO extras_mysql (id, codigo, lote) VALUES (3, 'em-nulos', 'A')`)
}

func setupSemSinal(t *testing.T, d *testDB) {
	t.Helper()
	d.mustExec(t, `CREATE TABLE sem_sinal (id bigint PRIMARY KEY, codigo varchar(20) NOT NULL UNIQUE, lote varchar(10) NOT NULL,
		c_u32 int unsigned, c_u64 bigint unsigned)`)
	d.mustExec(t, `INSERT INTO sem_sinal VALUES (1, 'ss-1', 'A', 4294967295, 18446744073709551615)`)
}

func setupExtrasSQLite(t *testing.T, d *testDB) {
	t.Helper()
	d.mustExec(t, `CREATE TABLE extras_sqlite (id INTEGER PRIMARY KEY, codigo VARCHAR(20) NOT NULL UNIQUE, lote VARCHAR(10) NOT NULL,
		c_desconhecido MY_TYPE, c_semtipo, c_bool1 TINYINT(1), c_big_sem_sinal UNSIGNED BIG INT, c_datahora DATETIME)`)
	d.mustExec(t, `INSERT INTO extras_sqlite VALUES (1, 'es-1', 'A', 'abc', 42, 1, 7, '2024-05-01T10:00:00Z')`)
	d.mustExec(t, `INSERT INTO extras_sqlite (id, codigo, lote, c_semtipo) VALUES (2, 'es-2', 'A', 'x')`)
	d.mustExec(t, `INSERT INTO extras_sqlite (id, codigo, lote) VALUES (3, 'es-nulos', 'A')`)
}

func setupNul(t *testing.T, d *testDB) {
	t.Helper()
	cols := []colDef{
		{name: "id", kind: "int", typ: tBigCol, pk: true},
		{name: "codigo", kind: "text", typ: tCod, notNull: true, unique: true},
		{name: "lote", kind: "text", typ: ct("varchar(10)", "varchar(10)", "VARCHAR(10)"), notNull: true},
		{name: "c_texto", kind: "text", typ: ct("text", "longtext", "TEXT")},
	}
	createTable(t, d, "nul_teste", cols, true)
	insert(t, d, "nul_teste", cols, map[string]any{"id": 1, "codigo": "nul-1", "lote": "A", "c_texto": "a\x00b"})
}

func setupFuso(t *testing.T, d *testDB) {
	t.Helper()
	d.mustExec(t, `CREATE TABLE fuso_teste (id INTEGER PRIMARY KEY, codigo VARCHAR(20) NOT NULL UNIQUE, lote VARCHAR(10) NOT NULL, c_ts TIMESTAMP)`)
	d.mustExec(t, `INSERT INTO fuso_teste VALUES (1, 'fuso-1', 'A', '2024-05-01T10:00:00+02:00')`)
}

// ---------------------------------------------------------------- grafos

var (
	categoriasCols = []colDef{
		{name: "id", kind: "int", typ: tBig, pk: true},
		{name: "codigo", kind: "text", typ: tCod, notNull: true, unique: true},
		{name: "nome", kind: "text", typ: tV40},
		{name: "pai_id", kind: "int", typ: tBig, fk: [2]string{"categorias", "id"}},
	}
	clientesCols = []colDef{
		{name: "id", kind: "int", typ: tBig, pk: true},
		{name: "codigo", kind: "text", typ: tCod, notNull: true, unique: true},
		{name: "nome", kind: "text", typ: tV40},
	}
	produtosCols = []colDef{
		{name: "id", kind: "int", typ: tBig, pk: true},
		{name: "codigo", kind: "text", typ: tCod, notNull: true, unique: true},
		{name: "nome", kind: "text", typ: tV40},
		{name: "preco", kind: "dec", typ: tDec102},
		{name: "categoria_id", kind: "int", typ: tBig, fk: [2]string{"categorias", "id"}},
	}
	pedidosCols = []colDef{
		{name: "id", kind: "int", typ: tBig, pk: true},
		{name: "codigo", kind: "text", typ: tCod, notNull: true, unique: true},
		{name: "cliente_id", kind: "int", typ: tBig, fk: [2]string{"clientes", "id"}},
		{name: "criado_em", kind: "ts", typ: tTs},
	}
	itensCols = []colDef{
		{name: "id", kind: "int", typ: tBig, pk: true},
		{name: "pedido_id", kind: "int", typ: tBig, fk: [2]string{"pedidos", "id"}},
		{name: "produto_id", kind: "int", typ: tBig, fk: [2]string{"produtos", "id"}},
		{name: "quantidade", kind: "int", typ: tInt},
	}
	pedidosLogCols = []colDef{
		{name: "id", kind: "int", typ: tBig, pk: true},
		{name: "pedido_id", kind: "int", typ: tBig, fk: [2]string{"pedidos", "id"}},
		{name: "evento", kind: "text", typ: tV40},
	}
)

func rows(t *testing.T, d *testDB, table string, cols []colDef, vals ...map[string]any) {
	t.Helper()
	for _, v := range vals {
		insert(t, d, table, cols, v)
	}
}

type m = map[string]any

func setupCategorias(t *testing.T, d *testDB) {
	t.Helper()
	createTable(t, d, "categorias", categoriasCols, true)
	rows(t, d, "categorias", categoriasCols,
		m{"id": 9, "codigo": "cat-raiz", "nome": "Raiz", "pai_id": nil},
		m{"id": 10, "codigo": "cat-filha", "nome": "Filha", "pai_id": 9},
		m{"id": 100, "codigo": "cat-neta", "nome": "Neta", "pai_id": 10})
}

func setupLoja(t *testing.T, d *testDB) {
	t.Helper()
	setupCategorias(t, d)
	createTable(t, d, "clientes", clientesCols, true)
	createTable(t, d, "produtos", produtosCols, true)
	createTable(t, d, "pedidos", pedidosCols, true)
	createTable(t, d, "itens", itensCols, true)
	createTable(t, d, "pedidos_log", pedidosLogCols, true)
	rows(t, d, "clientes", clientesCols,
		m{"id": 10, "codigo": "c-ana", "nome": "Ana"},
		m{"id": 20, "codigo": "c-bruno", "nome": "Bruno"})
	rows(t, d, "produtos", produtosCols,
		m{"id": 1, "codigo": "p-1", "nome": "Caneta", "preco": "3.50", "categoria_id": 100},
		m{"id": 2, "codigo": "p-2", "nome": "Caderno", "preco": "12.00", "categoria_id": 10})
	rows(t, d, "pedidos", pedidosCols,
		m{"id": 1001, "codigo": "ped-1", "cliente_id": 10, "criado_em": "2024-05-01 10:00:00"},
		m{"id": 1002, "codigo": "ped-2", "cliente_id": 20, "criado_em": "2024-05-02 11:00:00"})
	rows(t, d, "itens", itensCols,
		m{"id": 1, "pedido_id": 1001, "produto_id": 1, "quantidade": 2},
		m{"id": 2, "pedido_id": 1001, "produto_id": 2, "quantidade": 1},
		m{"id": 3, "pedido_id": 1002, "produto_id": 2, "quantidade": 5})
	rows(t, d, "pedidos_log", pedidosLogCols,
		m{"id": 1, "pedido_id": 1001, "evento": "criado"})
}

var (
	departamentosCols = []colDef{
		{name: "id", kind: "int", typ: tBig, pk: true},
		{name: "codigo", kind: "text", typ: tCod, notNull: true, unique: true},
		{name: "gerente_id", kind: "int", typ: tBig, fk: [2]string{"funcionarios", "id"}},
	}
	funcionariosCols = []colDef{
		{name: "id", kind: "int", typ: tBig, pk: true},
		{name: "codigo", kind: "text", typ: tCod, notNull: true, unique: true},
		{name: "departamento_id", kind: "int", typ: tBig, fk: [2]string{"departamentos", "id"}},
		{name: "chefe_id", kind: "int", typ: tBig, fk: [2]string{"funcionarios", "id"}},
	}
)

func setupCiclo(t *testing.T, d *testDB) {
	t.Helper()
	// SQLite aceita a referência adiante; Postgres e MySQL recebem as chaves depois dos dados.
	inline := d.eng.idx() == 2
	createTable(t, d, "departamentos", departamentosCols, inline)
	createTable(t, d, "funcionarios", funcionariosCols, inline)
	rows(t, d, "departamentos", departamentosCols, m{"id": 1, "codigo": "dep-ti", "gerente_id": 1})
	rows(t, d, "funcionarios", funcionariosCols,
		m{"id": 1, "codigo": "fun-ana", "departamento_id": 1, "chefe_id": nil},
		m{"id": 2, "codigo": "fun-bia", "departamento_id": 1, "chefe_id": 1},
		m{"id": 3, "codigo": "fun-caio", "departamento_id": 1, "chefe_id": 2})
	if !inline {
		addFKs(t, d, "departamentos", departamentosCols)
		addFKs(t, d, "funcionarios", funcionariosCols)
	}
}

var (
	contasCols = []colDef{
		{name: "id", kind: "int", typ: tBig, pk: true},
		{name: "codigo", kind: "text", typ: tCod, notNull: true, unique: true},
	}
	lancamentosCols = []colDef{
		{name: "id", kind: "int", typ: tBig, pk: true},
		{name: "conta_id", kind: "int", typ: tInt},
		{name: "valor", kind: "dec", typ: tDec102},
	}
)

func setupNome(t *testing.T, d *testDB) {
	t.Helper()
	createTable(t, d, "contas", contasCols, true)
	createTable(t, d, "lancamentos", lancamentosCols, true)
	rows(t, d, "contas", contasCols, m{"id": 1, "codigo": "conta-1"}, m{"id": 2, "codigo": "conta-2"})
	rows(t, d, "lancamentos", lancamentosCols,
		m{"id": 1, "conta_id": 1, "valor": "10.00"},
		m{"id": 2, "conta_id": 1, "valor": "20.00"},
		m{"id": 3, "conta_id": 2, "valor": "5.00"})
}

var (
	paisesCols = []colDef{
		{name: "sigla", kind: "text", typ: ct("varchar(2)", "varchar(2)", "VARCHAR(2)"), pk: true},
		{name: "nome", kind: "text", typ: tV40},
	}
	cidadesCols = []colDef{
		{name: "id", kind: "int", typ: tBig, pk: true},
		{name: "codigo", kind: "text", typ: tCod, notNull: true, unique: true},
		{name: "nome", kind: "text", typ: tV40},
		{name: "pais_sigla", kind: "text", typ: ct("varchar(2)", "varchar(2)", "VARCHAR(2)"), fk: [2]string{"paises", "sigla"}},
	}
)

func setupPKTexto(t *testing.T, d *testDB) {
	t.Helper()
	createTable(t, d, "paises", paisesCols, true)
	createTable(t, d, "cidades", cidadesCols, true)
	rows(t, d, "paises", paisesCols, m{"sigla": "BR", "nome": "Brasil"}, m{"sigla": "PT", "nome": "Portugal"})
	rows(t, d, "cidades", cidadesCols,
		m{"id": 5, "codigo": "cid-rj", "nome": "Rio", "pais_sigla": "BR"},
		m{"id": 7, "codigo": "cid-lis", "nome": "Lisboa", "pais_sigla": "PT"})
}

const nomeDificil = `ta"b ela`

func setupIdentificadores(t *testing.T, d *testDB) {
	t.Helper()
	cols := []colDef{
		{name: "id", kind: "int", typ: tBig, pk: true},
		{name: "group", kind: "text", typ: tCod},
		{name: `co"l`, kind: "text", typ: tCod},
		{name: "co`l", kind: "text", typ: tCod},
	}
	createTable(t, d, nomeDificil, cols, true)
	insert(t, d, nomeDificil, cols, m{"id": 1, "group": "g", `co"l`: "a", "co`l": "b"})
}
