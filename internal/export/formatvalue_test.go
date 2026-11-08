package export

import (
	"database/sql"
	"fmt"
	"strings"
	"testing"
)

// Os testes deste arquivo seguem docs/EXPORTACAO.md ("Como os testes devem conferir").

func fvNulo() sql.NullString { return sql.NullString{} }

func fvVal(s string) sql.NullString { return sql.NullString{String: s, Valid: true} }

var fvDialetos = []string{"postgres", "mysql", "sqlite"}

func fvNomeCategoria(c typeCategory) string {
	switch c {
	case categoryNumeric:
		return "numeric"
	case categoryBool:
		return "bool"
	case categoryBinary:
		return "binary"
	case categoryJSON:
		return "json"
	case categoryDatetime:
		return "datetime"
	case categoryText:
		return "text"
	}
	return fmt.Sprintf("desconhecida(%v)", c)
}

type fvClasse struct {
	nome string
	tipo string
	want typeCategory
}

func fvRodarClasses(t *testing.T, casos []fvClasse) {
	t.Helper()
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			if got := classifyType(c.tipo); got != c.want {
				t.Fatalf("classifyType(%q) = %s, quer %s", c.tipo, fvNomeCategoria(got), fvNomeCategoria(c.want))
			}
		})
	}
}

func TestClassifyType(t *testing.T) {
	fvRodarClasses(t, []fvClasse{
		{"int_minusculo", "int", categoryNumeric},
		{"integer_maiusculo", "INTEGER", categoryNumeric},
		{"int4_pgx", "INT4", categoryNumeric},
		{"bigint_misto", "BigInt", categoryNumeric},
		{"int_unsigned_sufixo", "INT UNSIGNED", categoryNumeric},
		{"unsigned_bigint_prefixo", "UNSIGNED BIGINT", categoryNumeric},
		{"tinyint_sem_tamanho", "TINYINT", categoryNumeric},
		{"numeric_com_parametros", "numeric(10,2)", categoryNumeric},
		{"decimal_com_parametros", "DECIMAL(10, 2)", categoryNumeric},
		{"double_precision", "double precision", categoryNumeric},
		{"double_precision_espacos", "Double  Precision", categoryNumeric},
		{"float8", "FLOAT8", categoryNumeric},
		{"real", "REAL", categoryNumeric},
		{"year_mysql", "YEAR", categoryNumeric},
		{"serial", "serial", categoryNumeric},
		{"bool_pgx", "BOOL", categoryBool},
		{"boolean", "boolean", categoryBool},
		{"tinyint_1_bool", "tinyint(1)", categoryBool},
		{"tinyint_1_maiusculo", "TINYINT(1)", categoryBool},
		{"tinyint_4_numerico", "tinyint(4)", categoryNumeric},
		{"bytea", "BYTEA", categoryBinary},
		{"blob", "BLOB", categoryBinary},
		{"longblob", "LONGBLOB", categoryBinary},
		{"varbinary_com_tamanho", "VARBINARY(16)", categoryBinary},
		{"binary_com_tamanho", "binary(16)", categoryBinary},
		{"json", "JSON", categoryJSON},
		{"jsonb", "JSONB", categoryJSON},
		{"date", "DATE", categoryDatetime},
		{"time_com_precisao", "time(3)", categoryDatetime},
		{"timestamp", "timestamp", categoryDatetime},
		{"timestamptz", "TIMESTAMPTZ", categoryDatetime},
		{"timestamp_with_time_zone", "timestamp(6) with time zone", categoryDatetime},
		{"datetime_mysql", "DATETIME", categoryDatetime},
		{"interval_nao_e_numero", "interval", categoryText},
		{"point_nao_e_numero", "point", categoryText},
		{"uuid", "UUID", categoryText},
		{"varchar_com_tamanho", "varchar(20)", categoryText},
		{"character_varying", "character varying", categoryText},
		{"text", "TEXT", categoryText},
		{"money", "money", categoryText},
		{"bit", "BIT", categoryText},
		{"array_prefixo_underscore", "_int4", categoryText},
		{"array_prefixo_underscore_texto", "_text", categoryText},
		{"array_sufixo_colchetes", "integer[]", categoryText},
		{"desconhecido", "my_custom_type", categoryText},
		{"desconhecido_com_int", "printer", categoryText},
		{"vazio", "", categoryText},
	})
}

// Listas fechadas do Passo 2, em minúsculas, maiúsculas e com espaços nas pontas.
func TestClassifyTypeListasDaDoc(t *testing.T) {
	listas := []struct {
		want  typeCategory
		nomes []string
	}{
		{categoryBool, []string{"bool", "boolean", "tinyint(1)"}},
		{categoryNumeric, []string{"int", "integer", "int1", "int2", "int3", "int4", "int8", "tinyint", "smallint", "mediumint", "bigint", "serial", "smallserial", "bigserial", "decimal", "dec", "numeric", "fixed", "float", "float4", "float8", "real", "double", "double precision", "year"}},
		{categoryBinary, []string{"bytea", "blob", "tinyblob", "mediumblob", "longblob", "binary", "varbinary"}},
		{categoryJSON, []string{"json", "jsonb"}},
		{categoryDatetime, []string{"date", "time", "timetz", "timestamp", "timestamptz", "datetime", "datetime2", "smalldatetime"}},
		{categoryText, []string{"interval", "point", "uuid", "inet", "cidr", "macaddr", "xml", "money", "bit", "varbit", "enum", "set", "char", "varchar", "character", "character varying", "bpchar", "name", "citext", "text", "tinytext", "mediumtext", "longtext", "clob", "geometry"}},
	}
	variacoes := []struct {
		nome string
		f    func(string) string
	}{
		{"original", func(s string) string { return s }},
		{"maiusculo", strings.ToUpper},
		{"espacos_nas_pontas", func(s string) string { return "  " + s + " \t" }},
	}
	var casos []fvClasse
	for _, l := range listas {
		for _, n := range l.nomes {
			for _, v := range variacoes {
				casos = append(casos, fvClasse{fvNomeCategoria(l.want) + "/" + n + "/" + v.nome, v.f(n), l.want})
			}
		}
	}
	fvRodarClasses(t, casos)
}

// Exemplos do Passo 1 (normalização) e nomes que apenas contêm um tipo conhecido.
func TestClassifyTypeNormalizacao(t *testing.T) {
	fvRodarClasses(t, []fvClasse{
		{"numeric_param_maiusculo", "NUMERIC(10,2)", categoryNumeric},
		{"varchar_param_maiusculo", "VARCHAR(20)", categoryText},
		{"text_com_espacos", "  Text ", categoryText},
		{"int_unsigned_misto", "Int Unsigned", categoryNumeric},
		{"int_signed", "int signed", categoryNumeric},
		{"int_unsigned_zerofill", "int unsigned zerofill", categoryNumeric},
		{"int_zerofill", "INT ZEROFILL", categoryNumeric},
		{"int_param_unsigned", "int(10) unsigned", categoryNumeric},
		{"bigint_param_unsigned", "bigint(20) UNSIGNED", categoryNumeric},
		{"timestamp_with_time_zone_sem_param", "timestamp with time zone", categoryDatetime},
		{"timestamp_without_time_zone", "timestamp without time zone", categoryDatetime},
		{"time_with_time_zone", "time with time zone", categoryDatetime},
		{"time_without_time_zone_param", "time(3) without time zone", categoryDatetime},
		{"timestamp_with_time_zone_maiusculo", "TIMESTAMP WITH TIME ZONE", categoryDatetime},
		{"varbinary_param", "varbinary(255)", categoryBinary},
		{"array_underscore_maiusculo", "_INT4", categoryText},
		{"array_colchetes_texto", "text[]", categoryText},
		{"array_colchetes_numeric", "NUMERIC[]", categoryText},
		{"array_colchetes_bool", "boolean[]", categoryText},
		{"array_underscore_bool", "_bool", categoryText},
		{"array_underscore_timestamp", "_timestamptz", categoryText},
		{"nome_com_int_no_meio", "pointer", categoryText},
		{"nome_com_int_prefixo", "internal", categoryText},
		{"nome_com_bool_no_inicio", "boolean_flag", categoryText},
		{"nome_com_date_no_meio", "update_info", categoryText},
		{"nome_com_json_no_inicio", "jsonpath", categoryText},
		{"nome_com_blob_no_fim", "blobby", categoryText},
		{"so_espacos", "   ", categoryText},
		{"tinyint_1_com_espacos", "  TinyInt(1) ", categoryBool},
	})
}

type fvCaso struct {
	nome    string
	dialeto string
	tipo    string
	valor   sql.NullString
	want    string
}

func fvRodar(t *testing.T, casos []fvCaso) {
	t.Helper()
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			got := formatValue(c.dialeto, c.tipo, c.valor)
			if got != c.want {
				t.Fatalf("formatValue(%q, %q, %+v) = %q, quer %q", c.dialeto, c.tipo, c.valor, got, c.want)
			}
		})
	}
}

// fvEmTodos repete cada caso nos três dialetos.
func fvEmTodos(casos []fvCaso) []fvCaso {
	var out []fvCaso
	for _, c := range casos {
		for _, d := range fvDialetos {
			n := c
			n.dialeto = d
			n.nome = d + "/" + c.nome
			out = append(out, n)
		}
	}
	return out
}

func TestFormatValueNuloEVazio(t *testing.T) {
	fvRodar(t, []fvCaso{
		{"nulo_numerico", "postgres", "int4", fvNulo(), "NULL"},
		{"nulo_texto", "mysql", "text", fvNulo(), "NULL"},
		{"nulo_bool", "sqlite", "bool", fvNulo(), "NULL"},
		{"nulo_binario", "postgres", "bytea", fvNulo(), "NULL"},
		{"nulo_json", "mysql", "json", fvNulo(), "NULL"},
		{"nulo_data", "sqlite", "timestamp", fvNulo(), "NULL"},
		{"texto_vazio", "postgres", "text", fvVal(""), "''"},
		{"numerico_vazio", "mysql", "int", fvVal(""), "''"},
	})
}

func TestFormatValueNuloEmTodasCategoriasEDialetos(t *testing.T) {
	var casos []fvCaso
	for _, tipo := range []string{"int4", "bool", "tinyint(1)", "bytea", "blob", "json", "jsonb", "timestamp", "date", "text", "interval", "_int4", "my_custom_type", ""} {
		casos = append(casos, fvCaso{"nulo_" + tipo, "", tipo, fvNulo(), "NULL"})
	}
	fvRodar(t, fvEmTodos(casos))
}

func TestFormatValueNumerico(t *testing.T) {
	fvRodar(t, fvEmTodos([]fvCaso{
		{"inteiro", "", "int4", fvVal("42"), "42"},
		{"inteiro_negativo", "", "bigint", fvVal("-7"), "-7"},
		{"unsigned_grande", "", "UNSIGNED BIGINT", fvVal("18446744073709551615"), "18446744073709551615"},
		{"decimal", "", "numeric(10,2)", fvVal("99.90"), "99.90"},
		{"decimal_sem_inteiro", "", "decimal", fvVal(".5"), ".5"},
		{"double_cientifico", "", "double precision", fvVal("1.5e10"), "1.5e10"},
		{"float_cientifico_negativo", "", "float8", fvVal("-2E-3"), "-2E-3"},
		{"year", "", "YEAR", fvVal("2024"), "2024"},
		{"nan", "", "float8", fvVal("NaN"), "'NaN'"},
		{"infinito", "", "float8", fvVal("Infinity"), "'Infinity'"},
		{"infinito_negativo", "", "float8", fvVal("-Infinity"), "'-Infinity'"},
		{"texto_no_numerico", "", "int", fvVal("1; DROP TABLE x"), "'1; DROP TABLE x'"},
		{"espaco_no_numerico", "", "int", fvVal(" 1"), "' 1'"},
	}))
}

// Variações da regex numérica da doc: ^-?(\d+(\.\d+)?|\.\d+)([eE][+-]?\d+)?$
func TestFormatValueNumericoRegex(t *testing.T) {
	fvRodar(t, fvEmTodos([]fvCaso{
		{"zero", "", "int", fvVal("0"), "0"},
		{"decimal_zero", "", "numeric", fvVal("0.0"), "0.0"},
		{"negativo_decimal", "", "numeric", fvVal("-3.14"), "-3.14"},
		{"negativo_sem_inteiro", "", "numeric", fvVal("-.5"), "-.5"},
		{"expoente_positivo", "", "float", fvVal("1e+5"), "1e+5"},
		{"expoente_maiusculo", "", "float", fvVal("2.5E10"), "2.5E10"},
		{"mais_explicito_nao_casa", "", "int", fvVal("+1"), "'+1'"},
		{"ponto_sem_digitos_depois", "", "numeric", fvVal("1."), "'1.'"},
		{"so_ponto", "", "numeric", fvVal("."), "'.'"},
		{"so_sinal", "", "int", fvVal("-"), "'-'"},
		{"expoente_sem_digitos", "", "float", fvVal("1e"), "'1e'"},
		{"hexadecimal", "", "int", fvVal("0x10"), "'0x10'"},
		{"virgula_decimal", "", "numeric", fvVal("1,5"), "'1,5'"},
		{"dois_pontos_decimais", "", "numeric", fvVal("1.2.3"), "'1.2.3'"},
		{"espaco_no_fim", "", "int", fvVal("1 "), "'1 '"},
		{"quebra_de_linha_no_fim", "", "int", fvVal("1\n"), "'1\n'"},
		{"aspas_no_numerico", "", "int", fvVal("1'2"), "'1''2'"},
		{"nan_minusculo", "", "float4", fvVal("nan"), "'nan'"},
		{"texto_em_decimal", "", "decimal(10,2)", fvVal("abc"), "'abc'"},
		{"texto_em_year", "", "year", fvVal("x"), "'x'"},
		{"unsigned_texto", "", "int unsigned", fvVal("1 OR 1=1"), "'1 OR 1=1'"},
	}))
}

func TestFormatValueNumericoPorTipoNormalizado(t *testing.T) {
	var casos []fvCaso
	for _, tipo := range []string{"INT UNSIGNED", "UNSIGNED BIGINT", "Double  Precision", "NUMERIC(10,2)", "tinyint(4)", "TINYINT", "smallint", "mediumint", "serial", "bigserial", "real", "float4", "dec", "fixed", "int1", "int2", "int3"} {
		casos = append(casos, fvCaso{"num_" + tipo, "", tipo, fvVal("123"), "123"})
	}
	fvRodar(t, fvEmTodos(casos))
}

func TestFormatValueNaoNumericosQueContinhamInt(t *testing.T) {
	fvRodar(t, fvEmTodos([]fvCaso{
		{"interval", "", "interval", fvVal("1 day"), "'1 day'"},
		{"interval_maiusculo", "", "INTERVAL", fvVal("2 years 3 mons"), "'2 years 3 mons'"},
		{"point", "", "point", fvVal("(1,2)"), "'(1,2)'"},
		{"desconhecido_com_int", "", "printer", fvVal("abc"), "'abc'"},
		{"desconhecido_com_int_valor_numerico", "", "printer", fvVal("42"), "'42'"},
		{"interval_valor_numerico", "", "interval", fvVal("1"), "'1'"},
		{"point_valor_numerico", "", "point", fvVal("5"), "'5'"},
		{"geometry", "", "geometry", fvVal("0101000000"), "'0101000000'"},
		{"bit_valor_numerico", "", "bit", fvVal("101"), "'101'"},
		{"varbit_valor_numerico", "", "varbit", fvVal("101"), "'101'"},
		{"money", "", "money", fvVal("12.50"), "'12.50'"},
		{"enum_valor_numerico", "", "enum", fvVal("1"), "'1'"},
		{"set_valor_numerico", "", "set", fvVal("1"), "'1'"},
		{"char_valor_numerico", "", "char(3)", fvVal("123"), "'123'"},
		{"inet", "", "inet", fvVal("10.0.0.1"), "'10.0.0.1'"},
		{"cidr", "", "cidr", fvVal("10.0.0.0/8"), "'10.0.0.0/8'"},
		{"macaddr", "", "macaddr", fvVal("08:00:2b:01:02:03"), "'08:00:2b:01:02:03'"},
		{"xml", "", "xml", fvVal("<a/>"), "'<a/>'"},
		{"citext", "", "citext", fvVal("Abc"), "'Abc'"},
		{"clob", "", "clob", fvVal("1"), "'1'"},
		{"mediumtext", "", "mediumtext", fvVal("1"), "'1'"},
		{"longtext", "", "longtext", fvVal("a'b"), "'a''b'"},
	}))
}

func TestFormatValueBooleano(t *testing.T) {
	fvRodar(t, []fvCaso{
		{"bool_t_postgres", "postgres", "bool", fvVal("t"), "TRUE"},
		{"bool_f_postgres", "postgres", "bool", fvVal("f"), "FALSE"},
		{"bool_true_mysql", "mysql", "boolean", fvVal("true"), "TRUE"},
		{"bool_false_mysql", "mysql", "boolean", fvVal("false"), "FALSE"},
		{"bool_1_mysql", "mysql", "tinyint(1)", fvVal("1"), "TRUE"},
		{"bool_0_mysql", "mysql", "tinyint(1)", fvVal("0"), "FALSE"},
		{"bool_true_sqlite", "sqlite", "boolean", fvVal("true"), "1"},
		{"bool_false_sqlite", "sqlite", "boolean", fvVal("false"), "0"},
		{"bool_1_sqlite", "sqlite", "BOOL", fvVal("1"), "1"},
		{"bool_0_sqlite", "sqlite", "BOOL", fvVal("0"), "0"},
		{"bool_maiusculo", "postgres", "bool", fvVal("TRUE"), "TRUE"},
		{"bool_T_maiusculo", "sqlite", "bool", fvVal("T"), "1"},
		{"bool_invalido", "postgres", "bool", fvVal("maybe"), "'maybe'"},
		{"bool_vazio", "mysql", "bool", fvVal(""), "''"},
		{"tinyint_numerico_mysql", "mysql", "TINYINT", fvVal("1"), "1"},
	})
}

func TestFormatValueBooleanoTodosOsDialetos(t *testing.T) {
	verdadeiros := []string{"t", "T", "true", "TRUE", "True", "1"}
	falsos := []string{"f", "F", "false", "FALSE", "False", "0"}
	tipos := []string{"bool", "BOOL", "boolean", "BOOLEAN", "tinyint(1)", "TINYINT(1)"}
	var casos []fvCaso
	for _, tipo := range tipos {
		for _, v := range verdadeiros {
			casos = append(casos,
				fvCaso{"verdadeiro/" + tipo + "/" + v + "/postgres", "postgres", tipo, fvVal(v), "TRUE"},
				fvCaso{"verdadeiro/" + tipo + "/" + v + "/mysql", "mysql", tipo, fvVal(v), "TRUE"},
				fvCaso{"verdadeiro/" + tipo + "/" + v + "/sqlite", "sqlite", tipo, fvVal(v), "1"},
			)
		}
		for _, v := range falsos {
			casos = append(casos,
				fvCaso{"falso/" + tipo + "/" + v + "/postgres", "postgres", tipo, fvVal(v), "FALSE"},
				fvCaso{"falso/" + tipo + "/" + v + "/mysql", "mysql", tipo, fvVal(v), "FALSE"},
				fvCaso{"falso/" + tipo + "/" + v + "/sqlite", "sqlite", tipo, fvVal(v), "0"},
			)
		}
	}
	for _, v := range []string{"maybe", "yes", "no", "2", "-1", " t", "t ", "", "tru", "on", "off"} {
		for _, d := range fvDialetos {
			casos = append(casos, fvCaso{"invalido/" + d + "/" + v, d, "bool", fvVal(v), "'" + v + "'"})
		}
	}
	fvRodar(t, casos)
}

func TestFormatValueBinario(t *testing.T) {
	fvRodar(t, []fvCaso{
		{"bytea_postgres", "postgres", "BYTEA", fvVal("Hi"), `'\x4869'`},
		{"blob_mysql", "mysql", "BLOB", fvVal("Hi"), "X'4869'"},
		{"blob_sqlite", "sqlite", "BLOB", fvVal("Hi"), "X'4869'"},
		{"binario_com_zero_e_alto", "mysql", "longblob", fvVal("\x00\xff\x27"), "X'00ff27'"},
		{"binario_com_zero_e_alto_pg", "postgres", "bytea", fvVal("\x00\xff\x27"), `'\x00ff27'`},
		{"varbinary_sqlite", "sqlite", "varbinary(16)", fvVal("\x01\x02"), "X'0102'"},
		{"binario_vazio_postgres", "postgres", "bytea", fvVal(""), `'\x'`},
		{"binario_vazio_mysql", "mysql", "blob", fvVal(""), "X''"},
		{"binario_vazio_sqlite", "sqlite", "blob", fvVal(""), "X''"},
	})
}

func TestFormatValueBinarioPorTipoEDialeto(t *testing.T) {
	var casos []fvCaso
	for _, tipo := range []string{"bytea", "BYTEA", "blob", "tinyblob", "mediumblob", "LONGBLOB", "binary", "binary(16)", "varbinary", "VARBINARY(16)"} {
		casos = append(casos,
			fvCaso{"postgres/" + tipo, "postgres", tipo, fvVal("Hi"), `'\x4869'`},
			fvCaso{"mysql/" + tipo, "mysql", tipo, fvVal("Hi"), "X'4869'"},
			fvCaso{"sqlite/" + tipo, "sqlite", tipo, fvVal("Hi"), "X'4869'"},
		)
	}
	// hex minúsculo, dois dígitos por byte; aspas viram hex, sem escape.
	casos = append(casos,
		fvCaso{"hex_minusculo_postgres", "postgres", "bytea", fvVal("\xab\xcd\xef"), `'\xabcdef'`},
		fvCaso{"hex_minusculo_mysql", "mysql", "blob", fvVal("\xab\xcd\xef"), "X'abcdef'"},
		fvCaso{"hex_zero_a_esquerda", "sqlite", "blob", fvVal("\x01\x0a"), "X'010a'"},
		fvCaso{"aspas_viram_hex_postgres", "postgres", "bytea", fvVal("it's"), `'\x69742773'`},
		fvCaso{"aspas_viram_hex_mysql", "mysql", "blob", fvVal("it's"), "X'69742773'"},
	)
	fvRodar(t, casos)
}

func TestFormatValueJSON(t *testing.T) {
	fvRodar(t, fvEmTodos([]fvCaso{
		{"json_objeto", "", "jsonb", fvVal(`{"a":1}`), `'{"a":1}'`},
		{"json_com_aspas_simples", "", "json", fvVal(`{"a":"it's"}`), `'{"a":"it''s"}'`},
		{"json_array", "", "JSON", fvVal(`[1,2,3]`), `'[1,2,3]'`},
		{"json_texto_nulo", "", "jsonb", fvVal("null"), `'null'`},
		{"json_numero_continua_texto", "", "json", fvVal("42"), `'42'`},
		{"json_true_continua_texto", "", "jsonb", fvVal("true"), `'true'`},
		{"json_vazio", "", "json", fvVal(""), `''`},
		{"json_maiusculo", "", "JSONB", fvVal(`{"k":"v"}`), `'{"k":"v"}'`},
	}))
}

func TestFormatValueArray(t *testing.T) {
	fvRodar(t, fvEmTodos([]fvCaso{
		{"array_inteiros", "", "_int4", fvVal("{1,2,3}"), "'{1,2,3}'"},
		{"array_inteiros_colchetes", "", "integer[]", fvVal("{1,2,3}"), "'{1,2,3}'"},
		{"array_texto_com_aspas", "", "_text", fvVal(`{"it's"}`), `'{"it''s"}'`},
		{"array_bool", "", "_bool", fvVal("{t,f}"), "'{t,f}'"},
		{"array_bool_colchetes", "", "boolean[]", fvVal("{t,f}"), "'{t,f}'"},
		{"array_numeric_colchetes", "", "numeric[]", fvVal("{1.5,2}"), "'{1.5,2}'"},
		{"array_vazio", "", "_int4", fvVal("{}"), "'{}'"},
		{"array_timestamp", "", "_timestamptz", fvVal("{2024-05-01T10:00:00Z}"), "'{2024-05-01T10:00:00Z}'"},
	}))
}

func TestFormatValueDataEHora(t *testing.T) {
	fvRodar(t, []fvCaso{
		{"data_postgres", "postgres", "date", fvVal("2024-05-01"), "'2024-05-01'"},
		{"timestamp_z_postgres", "postgres", "timestamptz", fvVal("2024-05-01T10:00:00Z"), "'2024-05-01T10:00:00Z'"},
		{"timestamp_z_mysql", "mysql", "TIMESTAMPTZ", fvVal("2024-05-01T10:00:00Z"), "'2024-05-01 10:00:00'"},
		{"timestamp_z_sqlite", "sqlite", "timestamp", fvVal("2024-05-01T10:00:00Z"), "'2024-05-01 10:00:00'"},
		{"timestamp_fracao_mysql", "mysql", "datetime", fvVal("2024-05-01T10:00:00.123456Z"), "'2024-05-01 10:00:00.123456'"},
		{"timestamp_deslocamento_mysql", "mysql", "timestamptz", fvVal("2024-05-01T10:00:00+02:00"), "'2024-05-01 10:00:00+02:00'"},
		{"timestamp_deslocamento_sqlite", "sqlite", "timestamptz", fvVal("2024-05-01T10:00:00-03:00"), "'2024-05-01 10:00:00-03:00'"},
		{"data_mysql", "mysql", "DATE", fvVal("2024-05-01"), "'2024-05-01'"},
		{"datetime_com_espaco_mysql", "mysql", "DATETIME", fvVal("2024-05-01 10:00:00"), "'2024-05-01 10:00:00'"},
		{"hora_sqlite", "sqlite", "time", fvVal("10:00:00"), "'10:00:00'"},
	})
}

func TestFormatValueDataEHoraVariacoes(t *testing.T) {
	fvRodar(t, []fvCaso{
		{"postgres_sem_ajuste_fracao", "postgres", "timestamp", fvVal("2024-05-01T10:00:00.5Z"), "'2024-05-01T10:00:00.5Z'"},
		{"postgres_sem_ajuste_deslocamento", "postgres", "timestamptz", fvVal("2024-05-01T10:00:00+02:00"), "'2024-05-01T10:00:00+02:00'"},
		{"postgres_sem_ajuste_sem_z", "postgres", "datetime", fvVal("2024-05-01T10:00:00"), "'2024-05-01T10:00:00'"},
		{"mysql_sem_z", "mysql", "datetime", fvVal("2024-05-01T10:00:00"), "'2024-05-01 10:00:00'"},
		{"sqlite_sem_z", "sqlite", "datetime", fvVal("2024-05-01T10:00:00"), "'2024-05-01 10:00:00'"},
		{"sqlite_fracao_z", "sqlite", "timestamp", fvVal("2024-05-01T10:00:00.123Z"), "'2024-05-01 10:00:00.123'"},
		{"sqlite_fracao_deslocamento", "sqlite", "timestamptz", fvVal("2024-05-01T10:00:00.5+02:00"), "'2024-05-01 10:00:00.5+02:00'"},
		{"mysql_data_sem_t", "mysql", "date", fvVal("2024-05-01"), "'2024-05-01'"},
		{"sqlite_data_sem_t", "sqlite", "date", fvVal("2024-05-01"), "'2024-05-01'"},
		{"postgres_hora", "postgres", "time", fvVal("10:00:00"), "'10:00:00'"},
		{"mysql_hora", "mysql", "time(3)", fvVal("10:00:00.123"), "'10:00:00.123'"},
		{"mysql_timestamp_with_time_zone", "mysql", "timestamp(6) with time zone", fvVal("2024-05-01T10:00:00Z"), "'2024-05-01 10:00:00'"},
		{"mysql_datetime2", "mysql", "datetime2", fvVal("2024-05-01T10:00:00Z"), "'2024-05-01 10:00:00'"},
		{"mysql_smalldatetime", "mysql", "smalldatetime", fvVal("2024-05-01T10:00:00Z"), "'2024-05-01 10:00:00'"},
		{"dialeto_desconhecido_ajusta_como_sqlite", "oracle", "timestamptz", fvVal("2024-05-01T10:00:00Z"), "'2024-05-01 10:00:00'"},
		{"data_nula_mysql", "mysql", "timestamptz", fvNulo(), "NULL"},
		{"data_vazia_mysql", "mysql", "timestamp", fvVal(""), "''"},
		{"data_vazia_postgres", "postgres", "date", fvVal(""), "''"},
	})
}

func TestFormatValueTextoEAspas(t *testing.T) {
	fvRodar(t, []fvCaso{
		{"texto_simples", "postgres", "text", fvVal("abc"), "'abc'"},
		{"texto_aspas_simples", "mysql", "varchar(20)", fvVal("it's"), "'it''s'"},
		{"texto_varias_aspas", "sqlite", "TEXT", fvVal("'a'b'"), "'''a''b'''"},
		{"texto_numerico_continua_texto", "postgres", "varchar(20)", fvVal("123"), "'123'"},
		{"uuid", "postgres", "uuid", fvVal("3f2b8c3e-0000-4000-8000-000000000000"), "'3f2b8c3e-0000-4000-8000-000000000000'"},
		{"desconhecido", "mysql", "my_custom_type", fvVal("42"), "'42'"},
		{"tipo_vazio", "sqlite", "", fvVal("42"), "'42'"},
	})
}

func TestFormatValueTextoTodosOsDialetos(t *testing.T) {
	fvRodar(t, fvEmTodos([]fvCaso{
		{"simples", "", "text", fvVal("abc"), "'abc'"},
		{"vazio", "", "text", fvVal(""), "''"},
		{"so_uma_aspa", "", "text", fvVal("'"), "''''"},
		{"aspas_duplas_inalteradas", "", "text", fvVal(`say "hi"`), `'say "hi"'`},
		{"acentos_inalterados", "", "varchar(10)", fvVal("ação é ü"), "'ação é ü'"},
		{"injecao_fica_dentro_das_aspas", "", "text", fvVal("a'; DROP TABLE x; --"), "'a''; DROP TABLE x; --'"},
		{"quebra_de_linha_inalterada", "", "text", fvVal("a\nb"), "'a\nb'"},
		{"nome_desconhecido_maiusculo", "", "MY_CUSTOM_TYPE", fvVal("1"), "'1'"},
		{"char_com_tamanho", "", "CHAR(3)", fvVal("ab"), "'ab'"},
		{"character_varying_param", "", "character varying(20)", fvVal("x"), "'x'"},
		{"tipo_so_espacos", "", "   ", fvVal("7"), "'7'"},
		{"uuid_maiusculo", "", "UUID", fvVal("A"), "'A'"},
	}))
}

func TestFormatValueNomeDoDialeto(t *testing.T) {
	fvRodar(t, []fvCaso{
		{"dialeto_pg", "pg", "bool", fvVal("t"), "TRUE"},
		{"dialeto_postgresql", "postgresql", "bytea", fvVal("Hi"), `'\x4869'`},
		{"dialeto_sqlite3", "sqlite3", "bool", fvVal("true"), "1"},
		{"dialeto_maiusculo", "MySQL", "blob", fvVal("Hi"), "X'4869'"},
		{"dialeto_desconhecido_usa_sqlite", "oracle", "bool", fvVal("true"), "1"},
	})
}

func TestFormatValueNomeDoDialetoVariacoes(t *testing.T) {
	fvRodar(t, []fvCaso{
		{"postgres_maiusculo_bool", "POSTGRES", "bool", fvVal("t"), "TRUE"},
		{"postgresql_misto_binario", "PostgreSQL", "bytea", fvVal("Hi"), `'\x4869'`},
		{"pg_maiusculo_binario", "PG", "bytea", fvVal("Hi"), `'\x4869'`},
		{"mysql_maiusculo_bool", "MYSQL", "bool", fvVal("1"), "TRUE"},
		{"sqlite_misto_bool", "SQLite", "bool", fvVal("1"), "1"},
		{"sqlite3_binario", "sqlite3", "blob", fvVal("Hi"), "X'4869'"},
		{"sqlite3_data", "sqlite3", "timestamptz", fvVal("2024-05-01T10:00:00Z"), "'2024-05-01 10:00:00'"},
		{"desconhecido_binario_como_sqlite", "oracle", "blob", fvVal("Hi"), "X'4869'"},
		{"desconhecido_bool_falso_como_sqlite", "oracle", "bool", fvVal("false"), "0"},
		{"desconhecido_nulo", "oracle", "text", fvNulo(), "NULL"},
		{"dialeto_vazio_como_sqlite_bool", "", "bool", fvVal("true"), "1"},
		{"dialeto_vazio_como_sqlite_binario", "", "blob", fvVal("Hi"), "X'4869'"},
	})
}

func TestFormatValueTinyintMySQLSemTamanho(t *testing.T) {
	// O driver do MySQL informa TINYINT sem o tamanho: numérico, sai 0/1 cru.
	fvRodar(t, fvEmTodos([]fvCaso{
		{"tinyint_sem_tamanho_1", "", "TINYINT", fvVal("1"), "1"},
		{"tinyint_sem_tamanho_0", "", "TINYINT", fvVal("0"), "0"},
		{"tinyint_4", "", "tinyint(4)", fvVal("7"), "7"},
	}))
}
