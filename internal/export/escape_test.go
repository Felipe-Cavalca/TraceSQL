package export

import (
	"database/sql"
	"fmt"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

// Os testes deste arquivo seguem docs/EXPORTACAO.md, seção "Escape do conteúdo (issue #63)"
// e "Ida e volta (round-trip)". Notação da doc: NUL, LF, CR, TAB e SUB são um só byte.

// escCaso é uma linha da tabela da doc: o valor lido e a saída esperada em cada dialeto.
type escCaso struct {
	nome   string
	tipo   string
	valor  string
	pg     string
	mysql  string
	sqlite string
}

func escRodar(t *testing.T, casos []escCaso) {
	t.Helper()
	for _, c := range casos {
		for _, d := range []struct{ nome, want string }{
			{"postgres", c.pg},
			{"mysql", c.mysql},
			{"sqlite", c.sqlite},
		} {
			t.Run(c.nome+"/"+d.nome, func(t *testing.T) {
				got := formatValue(d.nome, c.tipo, fvVal(c.valor))
				if got != d.want {
					t.Fatalf("formatValue(%q, %q, %q) = %q, quer %q", d.nome, c.tipo, c.valor, got, d.want)
				}
			})
		}
	}
}

// Os 22 casos de texto da doc (tipo text).
func TestEscapeConteudoTexto(t *testing.T) {
	escRodar(t, []escCaso{
		{"barra_simples", "text", `C:\temp`, `'C:\temp'`, `'C:\\temp'`, `'C:\temp'`},
		{"barra_dupla", "text", `a\\b`, `'a\\b'`, `'a\\\\b'`, `'a\\b'`},
		{"barra_no_fim", "text", `abc\`, `'abc\'`, `'abc\\'`, `'abc\'`},
		{"so_barra", "text", `\`, `'\'`, `'\\'`, `'\'`},
		{"barra_n_escrito", "text", `\n`, `'\n'`, `'\\n'`, `'\n'`},
		{"barra_zero_escrito", "text", `\0`, `'\0'`, `'\\0'`, `'\0'`},
		{"aspa_e_barra", "text", `it's \ ok`, `'it''s \ ok'`, `'it''s \\ ok'`, `'it''s \ ok'`},
		{"barra_antes_de_aspa", "text", `C:\temp\'x`, `'C:\temp\''x'`, `'C:\\temp\\''x'`, `'C:\temp\''x'`},
		{"injecao_aspa", "text", `'; DROP TABLE x;--`, `'''; DROP TABLE x;--'`, `'''; DROP TABLE x;--'`, `'''; DROP TABLE x;--'`},
		{"injecao_com_barra", "text", `\'; DROP TABLE x;--`, `'\''; DROP TABLE x;--'`, `'\\''; DROP TABLE x;--'`, `'\''; DROP TABLE x;--'`},
		{"nul", "text", "a\x00b", "'a\x00b'", `'a\0b'`, "'a\x00b'"},
		{"quebra_lf", "text", "l1\nl2", "'l1\nl2'", `'l1\nl2'`, "'l1\nl2'"},
		{"quebra_cr", "text", "l1\rl2", "'l1\rl2'", `'l1\rl2'`, "'l1\rl2'"},
		{"quebra_crlf", "text", "l1\r\nl2", "'l1\r\nl2'", `'l1\r\nl2'`, "'l1\r\nl2'"},
		{"barra_antes_de_lf", "text", "a\\\nb", "'a\\\nb'", `'a\\\nb'`, "'a\\\nb'"},
		{"ctrl_z", "text", "a\x1ab", "'a\x1ab'", `'a\Zb'`, "'a\x1ab'"},
		{"tabulacao", "text", "a\tb", "'a\tb'", "'a\tb'", "'a\tb'"},
		{"aspas_duplas", "text", `say "hi"`, `'say "hi"'`, `'say "hi"'`, `'say "hi"'`},
		{"unicode", "text", "ação 日本 😀", "'ação 日本 😀'", "'ação 日本 😀'", "'ação 日本 😀'"},
		{"unicode_com_barra", "text", `日\本`, `'日\本'`, `'日\\本'`, `'日\本'`},
		{"vazio", "text", "", `''`, `''`, `''`},
		{"sem_caractere_especial", "text", "abc", `'abc'`, `'abc'`, `'abc'`},
	})
}

// Casos que antes estavam em formatvalue_test.go com a mesma saída nos três
// dialetos; com a doc do #63 o mysql escapa a quebra de linha.
func TestEscapeQuebraDeLinhaPorDialeto(t *testing.T) {
	escRodar(t, []escCaso{
		{"texto_com_lf", "text", "a\nb", "'a\nb'", `'a\nb'`, "'a\nb'"},
		{"numerico_invalido_com_lf_no_fim", "int", "1\n", "'1\n'", `'1\n'`, "'1\n'"},
	})
}

// Outras categorias e caminhos que passam pelo mesmo escape.
func TestEscapeOutrasCategorias(t *testing.T) {
	escRodar(t, []escCaso{
		{"json_com_barra", "jsonb", `{"p":"C:\\temp"}`, `'{"p":"C:\\temp"}'`, `'{"p":"C:\\\\temp"}'`, `'{"p":"C:\\temp"}'`},
		{"json_com_aspa_e_barra", "json", `{"a":"it's \\"}`, `'{"a":"it''s \\"}'`, `'{"a":"it''s \\\\"}'`, `'{"a":"it''s \\"}'`},
		{"array_com_barra", "_text", `{"a\\b"}`, `'{"a\\b"}'`, `'{"a\\\\b"}'`, `'{"a\\b"}'`},
		{"desconhecido_com_barra", "my_custom_type", `\x41`, `'\x41'`, `'\\x41'`, `'\x41'`},
		{"data_fora_do_formato_com_barra", "datetime", `a\b`, `'a\b'`, `'a\\b'`, `'a\b'`},
		{"data_valida_nao_muda", "timestamp", "2024-05-01T10:00:00Z", "'2024-05-01T10:00:00Z'", "'2024-05-01 10:00:00'", "'2024-05-01 10:00:00'"},
		{"numerico_invalido_com_barra", "int", `1\2`, `'1\2'`, `'1\\2'`, `'1\2'`},
		{"bool_invalido_com_barra", "bool", `x\y`, `'x\y'`, `'x\\y'`, `'x\y'`},
	})

	// Binário não é escapado: o byte 0x5C vira hexadecimal. O tipo muda por dialeto.
	t.Run("binario_nao_e_escapado/postgres", func(t *testing.T) {
		if got, want := formatValue("postgres", "bytea", fvVal("\x5c")), `'\x5c'`; got != want {
			t.Fatalf("got %q, quer %q", got, want)
		}
	})
	for _, d := range []string{"mysql", "sqlite"} {
		t.Run("binario_nao_e_escapado/"+d, func(t *testing.T) {
			if got, want := formatValue(d, "blob", fvVal("\x5c")), "X'5c'"; got != want {
				t.Fatalf("got %q, quer %q", got, want)
			}
		})
	}

	for _, d := range fvDialetos {
		t.Run("nulo_continua_null/"+d, func(t *testing.T) {
			if got := formatValue(d, "text", fvNulo()); got != "NULL" {
				t.Fatalf("got %q, quer NULL", got)
			}
		})
	}
}

// Dialetos com o valor a\b'c do tipo text.
func TestEscapeNomeDoDialeto(t *testing.T) {
	casos := []struct{ nome, dialeto, want string }{
		{"dialeto_mysql_maiusculo", "MySQL", `'a\\b''c'`},
		{"dialeto_pg", "pg", `'a\b''c'`},
		{"dialeto_postgresql", "postgresql", `'a\b''c'`},
		{"dialeto_sqlite3", "sqlite3", `'a\b''c'`},
		{"dialeto_desconhecido", "oracle", `'a\b''c'`},
		{"dialeto_vazio", "", `'a\b''c'`},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			if got := formatValue(c.dialeto, "text", fvVal(`a\b'c`)); got != c.want {
				t.Fatalf("formatValue(%q, text, a\\b'c) = %q, quer %q", c.dialeto, got, c.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Ida e volta.
// ---------------------------------------------------------------------------

// Conjunto mínimo da doc. O item com NUL fica fora do postgres.
func escValoresIdaEVolta(comNUL bool) []string {
	vs := []string{
		"abc", `C:\temp`, `a\\b`, `abc\`, `\`, "it's", `C:\temp\'x`,
		`'; DROP TABLE x;--`, `\'; DROP TABLE x;--`,
		"l1\nl2", "l1\r\nl2", "a\tb", "a\x1ab", `say "hi"`, "ação 日本 😀", `\n`, "",
	}
	if comNUL {
		vs = append(vs, "a\x00b")
	}
	return vs
}

func escNomeValor(v string) string {
	return fmt.Sprintf("%q", v)
}

// escDecodificaMySQL aplica a regra de leitura do mysql descrita na doc.
func escDecodificaMySQL(lit string) (string, error) {
	if len(lit) < 2 || lit[0] != '\'' || lit[len(lit)-1] != '\'' {
		return "", fmt.Errorf("literal sem aspas externas: %q", lit)
	}
	in := lit[1 : len(lit)-1]
	var out strings.Builder
	for i := 0; i < len(in); i++ {
		c := in[i]
		switch c {
		case '\'':
			if i+1 < len(in) && in[i+1] == '\'' {
				out.WriteByte('\'')
				i++
				continue
			}
			return "", fmt.Errorf("aspa simples sozinha na posição %d de %q", i, lit)
		case '\\':
			if i+1 >= len(in) {
				return "", fmt.Errorf("barra no fim sem caractere seguinte em %q", lit)
			}
			i++
			switch in[i] {
			case '0':
				out.WriteByte(0)
			case 'n':
				out.WriteByte('\n')
			case 'r':
				out.WriteByte('\r')
			case 'Z':
				out.WriteByte(0x1a)
			default: // \\ e \' valem o próprio caractere; os demais também
				out.WriteByte(in[i])
			}
		default:
			out.WriteByte(c)
		}
	}
	return out.String(), nil
}

// escDecodificaPostgres tira as aspas externas e troca '' por '; a barra não tem significado.
func escDecodificaPostgres(lit string) (string, error) {
	if len(lit) < 2 || lit[0] != '\'' || lit[len(lit)-1] != '\'' {
		return "", fmt.Errorf("literal sem aspas externas: %q", lit)
	}
	in := lit[1 : len(lit)-1]
	for i := 0; i < len(in); i++ {
		if in[i] == '\'' {
			if i+1 < len(in) && in[i+1] == '\'' {
				i++
				continue
			}
			return "", fmt.Errorf("aspa simples sozinha na posição %d de %q", i, lit)
		}
	}
	return strings.ReplaceAll(in, "''", "'"), nil
}

func TestIdaEVoltaMySQL(t *testing.T) {
	for _, v := range escValoresIdaEVolta(true) {
		t.Run(escNomeValor(v), func(t *testing.T) {
			lit := formatValue("mysql", "text", fvVal(v))
			got, err := escDecodificaMySQL(lit)
			if err != nil {
				t.Fatalf("literal %q inválido para o mysql: %v", lit, err)
			}
			if got != v {
				t.Fatalf("ida e volta: literal %q lido como %q, quer %q", lit, got, v)
			}
		})
	}
}

func TestIdaEVoltaPostgres(t *testing.T) {
	for _, v := range escValoresIdaEVolta(false) {
		t.Run(escNomeValor(v), func(t *testing.T) {
			lit := formatValue("postgres", "text", fvVal(v))
			got, err := escDecodificaPostgres(lit)
			if err != nil {
				t.Fatalf("literal %q inválido para o postgres: %v", lit, err)
			}
			if got != v {
				t.Fatalf("ida e volta: literal %q lido como %q, quer %q", lit, got, v)
			}
		})
	}
}

func escAbrirSQLite(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("abrir sqlite: %v", err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	return db
}

func TestIdaEVoltaSQLiteSelect(t *testing.T) {
	db := escAbrirSQLite(t)
	for _, v := range escValoresIdaEVolta(true) {
		t.Run(escNomeValor(v), func(t *testing.T) {
			lit := formatValue("sqlite", "text", fvVal(v))
			var got string
			if err := db.QueryRow("SELECT " + lit).Scan(&got); err != nil {
				t.Fatalf("SELECT %s: %v", lit, err)
			}
			if got != v {
				t.Fatalf("ida e volta: literal %q lido como %q, quer %q", lit, got, v)
			}
		})
	}
}

func TestIdaEVoltaSQLiteInsertSelect(t *testing.T) {
	db := escAbrirSQLite(t)
	if _, err := db.Exec("CREATE TABLE t (id INTEGER PRIMARY KEY, v TEXT)"); err != nil {
		t.Fatalf("criar tabela: %v", err)
	}
	for i, v := range escValoresIdaEVolta(true) {
		t.Run(escNomeValor(v), func(t *testing.T) {
			lit := formatValue("sqlite", "text", fvVal(v))
			if _, err := db.Exec(fmt.Sprintf("INSERT INTO t (id, v) VALUES (%d, %s)", i, lit)); err != nil {
				t.Fatalf("INSERT com %s: %v", lit, err)
			}
			var got string
			if err := db.QueryRow("SELECT v FROM t WHERE id = ?", i).Scan(&got); err != nil {
				t.Fatalf("SELECT: %v", err)
			}
			if got != v {
				t.Fatalf("ida e volta: literal %q lido como %q, quer %q", lit, got, v)
			}
		})
	}
}

// Nenhuma aspa simples sozinha dentro do literal, em nenhum dialeto: o texto
// não consegue fechar o literal. A contagem por dialeto considera o escape de
// barra do mysql (a aspa depois de uma barra de escape ainda precisa de par).
func TestEscapeNenhumaAspaSozinha(t *testing.T) {
	for _, d := range fvDialetos {
		for _, v := range escValoresIdaEVolta(d != "postgres") {
			t.Run(d+"/"+escNomeValor(v), func(t *testing.T) {
				lit := formatValue(d, "text", fvVal(v))
				var err error
				if d == "mysql" {
					_, err = escDecodificaMySQL(lit)
				} else {
					_, err = escDecodificaPostgres(lit)
				}
				if err != nil {
					t.Fatalf("literal %q: %v", lit, err)
				}
			})
		}
	}
}

// Teste negativo: o literal que o código antigo gerava (só dobrava a aspa)
// não é lido de volta como o original pelo decodificador do mysql.
func TestDecodificadorMySQLRejeitaLiteralDoCodigoAntigo(t *testing.T) {
	antigo := func(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }

	if got := antigo(`C:\temp\'x`); got != `'C:\temp\''x'` {
		t.Fatalf("literal antigo inesperado: %q", got)
	}
	for _, v := range []string{`C:\temp\'x`, `C:\temp`, `abc\`, `\'; DROP TABLE x;--`, `\0`} {
		t.Run(escNomeValor(v), func(t *testing.T) {
			got, err := escDecodificaMySQL(antigo(v))
			if err == nil && got == v {
				t.Fatalf("o decodificador aceitou o literal antigo %q como %q; o teste não pega o erro", antigo(v), v)
			}
		})
	}

	// O literal correto, ao contrário, é aceito (a rejeição não é cega).
	got, err := escDecodificaMySQL(`'C:\\temp\\''x'`)
	if err != nil || got != `C:\temp\'x` {
		t.Fatalf("literal correto rejeitado: %q, %v", got, err)
	}
}
