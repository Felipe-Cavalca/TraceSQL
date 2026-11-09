package export

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math/big"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/Felipe-Cavalca/TraceSQL/internal/metadata"
)

// ---------------------------------------------------------------- tipos

// itTipoCanonico reduz o tipo declarado a uma forma comparável entre dialetos.
// Com sqlite envolvido, tamanhos de inteiro e fração de data são ignorados (o
// SQLite só tem afinidades).
func itTipoCanonico(raw string, comSQLite bool) string {
	l := strings.ToLower(strings.TrimSpace(raw))
	params := ""
	if i := strings.Index(l, "("); i >= 0 {
		if j := strings.Index(l[i:], ")"); j >= 0 {
			params = strings.ReplaceAll(l[i+1:i+j], " ", "")
			l = strings.TrimSpace(l[:i] + l[i+j+1:])
		}
	}
	switch {
	case l == "bool", l == "boolean", l == "tinyint" && params == "1":
		return "bool"
	case strings.HasPrefix(l, "bigint"):
		if comSQLite {
			return "int"
		}
		return "bigint"
	case strings.Contains(l, "int") && !strings.Contains(l, "interval") && !strings.Contains(l, "point"):
		return "int"
	case l == "decimal" || l == "numeric" || l == "dec":
		return "dec(" + params + ")"
	case strings.HasPrefix(l, "double"):
		if comSQLite {
			return "float"
		}
		return "double"
	case l == "real" || l == "float" || l == "float4":
		return "float"
	case l == "float8":
		return "double"
	case l == "varchar" || l == "character varying":
		return "varchar(" + params + ")"
	case l == "char" || l == "character" || l == "bpchar":
		return "char(" + params + ")"
	case strings.HasSuffix(l, "text") || strings.Contains(l, "clob"):
		return "text"
	case strings.Contains(l, "blob") || l == "bytea" || l == "binary" || l == "varbinary":
		return "bin"
	case strings.HasPrefix(l, "json"):
		return "json"
	case strings.HasPrefix(l, "timestamp") || strings.HasPrefix(l, "datetime"):
		if comSQLite {
			return "ts"
		}
		return "ts(" + params + ")"
	case l == "date":
		return "date"
	case strings.HasPrefix(l, "time"):
		return "time"
	case l == "uuid":
		return "uuid"
	}
	return l
}

func itFamilia(canon string) string {
	if i := strings.Index(canon, "("); i >= 0 {
		return canon[:i]
	}
	return canon
}

// itTipoEquivalente diz se o tipo da origem pode legitimamente virar o do
// destino. Mudanças esperadas entre dialetos ficam nesta tabela.
func itTipoEquivalente(origem, destino itMotor, tipoOrigem, tipoDestino string) bool {
	comSQLite := origem.driver == "sqlite" || destino.driver == "sqlite"
	o, d := itTipoCanonico(tipoOrigem, comSQLite), itTipoCanonico(tipoDestino, comSQLite)
	if o == d {
		return true
	}
	fo, fd := itFamilia(o), itFamilia(d)
	switch {
	case fo == "dec" && fd == "dec" && (strings.HasSuffix(o, "()") || strings.HasSuffix(d, "()")):
		return true // numeric sem precisão: qualquer precisão do outro lado
	case fo == "json" && fd == "text" && destino.driver == "sqlite":
		return true // o SQLite não tem JSON
	case fo == "ts" && fd == "ts" && ((destino.driver == "postgres" && d == "ts()" && o == "ts(6)") || (origem.driver == "postgres" && o == "ts()" && d == "ts(6)")):
		return true // no Postgres, timestamp sem precisão é timestamp(6)
	case fo == "uuid" && (fd == "text" || fd == "char" || fd == "varchar"):
		return true
	}
	return false
}

// ---------------------------------------------------------------- leitura

func itDescobre(ctx context.Context, b *itBanco) (metadata.Catalog, error) {
	return metadata.Discover(ctx, b.db, b.motor.driver)
}

func itLinhas(ctx context.Context, b *itBanco, tabela string) ([]string, []map[string]sql.NullString, error) {
	rows, err := b.db.QueryContext(ctx, "SELECT * FROM "+itQuote(b.motor, tabela))
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		return nil, nil, err
	}
	var out []map[string]sql.NullString
	for rows.Next() {
		dest := make([]sql.NullString, len(cols))
		ptrs := make([]any, len(cols))
		for i := range dest {
			ptrs[i] = &dest[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, nil, err
		}
		m := make(map[string]sql.NullString, len(cols))
		for i, c := range cols {
			m[c] = dest[i]
		}
		out = append(out, m)
	}
	return cols, out, rows.Err()
}

// ---------------------------------------------------------------- valores

const itNulo = "\xffNULL"

func itTempo(familia, s string) string {
	s = strings.Replace(s, "T", " ", 1)
	s = strings.TrimSuffix(s, "Z")
	s = strings.TrimSuffix(s, "+00:00")
	if i := strings.Index(s, "."); i >= 0 {
		j := i + 1
		for j < len(s) && s[j] >= '0' && s[j] <= '9' {
			j++
		}
		frac := strings.TrimRight(s[i:j], "0")
		if frac == "." {
			frac = ""
		}
		s = s[:i] + frac + s[j:]
	}
	if familia == "date" {
		s = strings.TrimSuffix(s, " 00:00:00")
	}
	return s
}

// itValor normaliza o valor lido conforme a família do tipo da ORIGEM, para
// que a mesma regra valha nos dois lados da comparação.
func itValor(familia string, v sql.NullString) any {
	if !v.Valid {
		return itNulo
	}
	s := v.String
	switch familia {
	case "bool":
		switch strings.ToLower(s) {
		case "t", "true", "1":
			return "1"
		case "f", "false", "0":
			return "0"
		}
	case "dec":
		if r, ok := new(big.Rat).SetString(s); ok {
			return r.RatString()
		}
	case "float", "double":
		if f, err := strconv.ParseFloat(s, 64); err == nil {
			return strconv.FormatFloat(f, 'g', -1, 64)
		}
	case "json":
		var j any
		if err := json.Unmarshal([]byte(s), &j); err == nil {
			return j
		}
	case "date", "time", "ts":
		return itTempo(familia, s)
	}
	return s
}

// ---------------------------------------------------------------- comparação

// itCompara confere schema (nomes, colunas, nulabilidade, tipos mapeados, PKs e
// FKs) e dados (linha a linha, ligando as linhas pela chave natural `codigo`).
// novosIDs: a PK não é comparada, só a ligação entre as linhas.
func itCompara(t *testing.T, ctx context.Context, orig, dest *itBanco, esperado map[string]int, novosIDs, semFKs bool) {
	t.Helper()
	catO, err := itDescobre(ctx, orig)
	if err != nil {
		t.Fatalf("descobrir schema da origem: %v", err)
	}
	catD, err := itDescobre(ctx, dest)
	if err != nil {
		t.Fatalf("descobrir schema do destino: %v", err)
	}

	// Tabelas.
	var nomesD []string
	for _, n := range catD.TableNames() {
		if !strings.HasPrefix(n, "__tracesql_map_") && n != "sqlite_sequence" {
			nomesD = append(nomesD, n)
		}
	}
	sort.Strings(nomesD)
	var quer []string
	for n := range esperado {
		quer = append(quer, n)
	}
	sort.Strings(quer)
	if !reflect.DeepEqual(nomesD, quer) {
		t.Errorf("tabelas no destino: %q, esperado %q", nomesD, quer)
		return
	}

	// Colunas.
	for _, nome := range nomesD {
		tO, ok := catO.Table(nome)
		if !ok {
			t.Errorf("tabela %s não existe na origem", nome)
			continue
		}
		tD, _ := catD.Table(nome)
		colsD := map[string]metadata.Column{}
		for _, c := range tD.Columns {
			colsD[c.Name] = c
		}
		if len(tD.Columns) != len(tO.Columns) {
			t.Errorf("%s: %d colunas no destino, %d na origem", nome, len(tD.Columns), len(tO.Columns))
		}
		for _, cO := range tO.Columns {
			cD, ok := colsD[cO.Name]
			if !ok {
				t.Errorf("%s.%s: coluna ausente no destino", nome, cO.Name)
				continue
			}
			if cO.PrimaryKey != cD.PrimaryKey {
				t.Errorf("%s.%s: PK origem=%v destino=%v", nome, cO.Name, cO.PrimaryKey, cD.PrimaryKey)
			}
			if (cO.Nullable && !cO.PrimaryKey) != (cD.Nullable && !cD.PrimaryKey) {
				t.Errorf("%s.%s: nulável origem=%v destino=%v", nome, cO.Name, cO.Nullable, cD.Nullable)
			}
			if !itColunaLivre(nome, cO.Name) && !itTipoEquivalente(orig.motor, dest.motor, cO.Type, cD.Type) {
				t.Errorf("%s.%s: tipo %q na origem virou %q no destino (não é equivalente)", nome, cO.Name, cO.Type, cD.Type)
			}
		}
	}

	// FKs cujo pai foi exportado.
	if !semFKs {
		chave := func(fk metadata.ForeignKey) string {
			return strings.ToLower(fk.Table + "(" + strings.Join(fk.Columns, ",") + ")->" + fk.RefTable + "(" + strings.Join(fk.RefColumns, ",") + ")")
		}
		var fkO, fkD []string
		for _, fk := range catO.ForeignKeys {
			if _, a := esperado[fk.Table]; a {
				if _, b := esperado[fk.RefTable]; b {
					fkO = append(fkO, chave(fk))
				}
			}
		}
		for _, fk := range catD.ForeignKeys {
			fkD = append(fkD, chave(fk))
		}
		sort.Strings(fkO)
		sort.Strings(fkD)
		if !reflect.DeepEqual(fkO, fkD) {
			t.Errorf("chaves estrangeiras: destino %q, esperado %q", fkD, fkO)
		}
	}

	// Dados.
	type dados struct {
		cols []string
		rows []map[string]sql.NullString
	}
	lerTudo := func(b *itBanco, cat metadata.Catalog, tabelas []string) (map[string]dados, error) {
		out := map[string]dados{}
		for _, n := range tabelas {
			c, r, err := itLinhas(ctx, b, n)
			if err != nil {
				return nil, fmt.Errorf("ler %s: %w", n, err)
			}
			out[n] = dados{c, r}
		}
		return out, nil
	}
	// A origem é lida inteira (os pais fora do grafo exportado também entram na tradução de FK).
	todasO := catO.TableNames()
	dO, err := lerTudo(orig, catO, todasO)
	if err != nil {
		t.Fatalf("%v", err)
	}
	dD, err := lerTudo(dest, catD, nomesD)
	if err != nil {
		t.Fatalf("%v", err)
	}

	idParaCodigo := func(d map[string]dados, cat metadata.Catalog) map[string]map[string]string {
		out := map[string]map[string]string{}
		for n, tab := range d {
			tb, _ := cat.Table(n)
			pk := tb.PrimaryKeyColumns()
			if len(pk) != 1 {
				continue
			}
			out[n] = map[string]string{}
			for _, r := range tab.rows {
				out[n][r[pk[0]].String] = r["codigo"].String
			}
		}
		return out
	}
	mapaO, mapaD := idParaCodigo(dO, catO), idParaCodigo(dD, catD)

	fkPorColuna := map[string]string{} // "tabela.coluna" -> tabela referenciada (FK simples)
	for _, fk := range catO.ForeignKeys {
		if len(fk.Columns) == 1 {
			fkPorColuna[fk.Table+"."+fk.Columns[0]] = fk.RefTable
		}
	}
	if semFKs {
		// FK inferida por nome: [tabela]_id -> tabela (singular simples).
		for _, n := range nomesD {
			tab, _ := catO.Table(n)
			for _, c := range tab.Columns {
				if ref, ok := strings.CutSuffix(c.Name, "_id"); ok {
					for _, cand := range todasO {
						if cand == ref+"s" || cand == ref {
							fkPorColuna[n+"."+c.Name] = cand
						}
					}
				}
			}
		}
	}

	for _, nome := range nomesD {
		tabO, _ := catO.Table(nome)
		pk := tabO.PrimaryKeyColumns()
		if got := len(dD[nome].rows); got != esperado[nome] {
			t.Errorf("%s: %d linhas no destino, esperado %d", nome, got, esperado[nome])
		}
		fam := map[string]string{}
		for _, c := range tabO.Columns {
			fam[c.Name] = itFamilia(itTipoCanonico(c.Type, true))
		}
		porCodigoO := map[string]map[string]sql.NullString{}
		for _, r := range dO[nome].rows {
			porCodigoO[r["codigo"].String] = r
		}
		for _, rD := range dD[nome].rows {
			cod := rD["codigo"].String
			rO, ok := porCodigoO[cod]
			if !ok {
				t.Errorf("%s: linha %q do destino não existe na origem", nome, cod)
				continue
			}
			for _, c := range tabO.Columns {
				vO, vD := itValor(fam[c.Name], rO[c.Name]), itValor(fam[c.Name], rD[c.Name])
				if ref, ok := fkPorColuna[nome+"."+c.Name]; ok && itContem(nomesD, ref) {
					vO, vD = itNulo, itNulo
					if rO[c.Name].Valid {
						vO = mapaO[ref][rO[c.Name].String]
					}
					if rD[c.Name].Valid {
						vD = mapaD[ref][rD[c.Name].String]
					}
				} else if novosIDs && len(pk) == 1 && c.Name == pk[0] {
					continue
				}
				if !reflect.DeepEqual(vO, vD) {
					t.Errorf("%s[%s].%s: origem %s, destino %s", nome, cod, c.Name, itMostra(rO[c.Name]), itMostra(rD[c.Name]))
				}
			}
		}
	}
}

func itMostra(v sql.NullString) string {
	if !v.Valid {
		return "NULL"
	}
	if len(v.String) > 60 {
		return fmt.Sprintf("%q...(%d bytes)", v.String[:60], len(v.String))
	}
	return fmt.Sprintf("%q", v.String)
}

// itColunaLivre marca colunas dos extras por motor, cujo tipo muda de forma
// legítima e imprevisível entre dialetos (interval, point, enum, unsigned...).
func itColunaLivre(tabela, coluna string) bool {
	switch tabela {
	case "extras_pg", "extras_pg_intervalo", "extras_mysql", "extras_mysql_sem_sinal", "extras_sqlite":
		return strings.HasPrefix(coluna, "c_")
	}
	return false
}
