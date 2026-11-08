package export

import (
	"database/sql"
	"encoding/hex"
	"regexp"
	"strings"
)

// typeCategory agrupa os tipos de coluna pelo literal SQL que cada um recebe.
type typeCategory int

const (
	categoryText typeCategory = iota
	categoryNumeric
	categoryBool
	categoryBinary
	categoryJSON
	categoryDatetime
)

// typeCategories é a lista fechada de nomes normalizados. Nome fora da lista é texto.
var typeCategories = map[string]typeCategory{
	"bool": categoryBool, "boolean": categoryBool, "tinyint(1)": categoryBool,

	"int": categoryNumeric, "integer": categoryNumeric, "int1": categoryNumeric, "int2": categoryNumeric,
	"int3": categoryNumeric, "int4": categoryNumeric, "int8": categoryNumeric, "tinyint": categoryNumeric,
	"smallint": categoryNumeric, "mediumint": categoryNumeric, "bigint": categoryNumeric,
	"serial": categoryNumeric, "smallserial": categoryNumeric, "bigserial": categoryNumeric,
	"decimal": categoryNumeric, "dec": categoryNumeric, "numeric": categoryNumeric, "fixed": categoryNumeric,
	"float": categoryNumeric, "float4": categoryNumeric, "float8": categoryNumeric, "real": categoryNumeric,
	"double": categoryNumeric, "double precision": categoryNumeric, "year": categoryNumeric,

	"bytea": categoryBinary, "blob": categoryBinary, "tinyblob": categoryBinary, "mediumblob": categoryBinary,
	"longblob": categoryBinary, "binary": categoryBinary, "varbinary": categoryBinary,

	"json": categoryJSON, "jsonb": categoryJSON,

	"date": categoryDatetime, "time": categoryDatetime, "timetz": categoryDatetime, "timestamp": categoryDatetime,
	"timestamptz": categoryDatetime, "datetime": categoryDatetime, "datetime2": categoryDatetime,
	"smalldatetime": categoryDatetime,
}

var (
	typeParensPattern    = regexp.MustCompile(`\([^)]*\)`)
	typeModifiersPattern = regexp.MustCompile(`\b(unsigned|signed|zerofill)\b`)
	typeSpacesPattern    = regexp.MustCompile(`\s+`)
	numericLiteral       = regexp.MustCompile(`^-?(\d+(\.\d+)?|\.\d+)([eE][+-]?\d+)?$`)
	isoDatetimePattern   = regexp.MustCompile(`^(\d{4}-\d{2}-\d{2})T(\d{2}:\d{2}:\d{2}(?:\.\d+)?)(Z|[+-]\d{2}:\d{2})?$`)
)

// normalizeTypeName aplica o Passo 1 de docs/EXPORTACAO.md.
func normalizeTypeName(typeName string) string {
	name := strings.ToLower(strings.TrimSpace(typeName))
	if name != "tinyint(1)" {
		name = typeParensPattern.ReplaceAllString(name, "")
	}
	name = typeModifiersPattern.ReplaceAllString(name, "")
	name = strings.TrimSpace(name)
	name = strings.TrimSuffix(name, "without time zone")
	name = strings.TrimSuffix(name, "with time zone")
	return strings.TrimSpace(typeSpacesPattern.ReplaceAllString(name, " "))
}

// classifyType devolve a categoria do tipo. Nome vazio, array ou desconhecido é texto.
func classifyType(typeName string) typeCategory {
	raw := strings.ToLower(strings.TrimSpace(typeName))
	if strings.HasPrefix(raw, "_") || strings.HasSuffix(raw, "[]") {
		return categoryText
	}
	if category, ok := typeCategories[normalizeTypeName(typeName)]; ok {
		return category
	}
	return categoryText
}

// formatValue escreve o valor lido como literal SQL conforme a categoria do tipo e o dialeto de saída.
func formatValue(outputDriver, typeName string, v sql.NullString) string {
	if !v.Valid {
		return "NULL"
	}

	dialect := normalizeDialect(outputDriver)
	if dialect != "postgres" && dialect != "mysql" {
		dialect = "sqlite"
	}

	switch classifyType(typeName) {
	case categoryNumeric:
		if numericLiteral.MatchString(v.String) {
			return v.String
		}
	case categoryBool:
		if literal, ok := boolLiteral(dialect, v.String); ok {
			return literal
		}
	case categoryBinary:
		encoded := hex.EncodeToString([]byte(v.String))
		if dialect == "postgres" {
			return `'\x` + encoded + "'"
		}
		return "X'" + encoded + "'"
	case categoryDatetime:
		if dialect != "postgres" {
			return quoteText(dialect, adjustDatetime(v.String))
		}
	}

	return quoteText(dialect, v.String)
}

func boolLiteral(dialect, value string) (string, bool) {
	var truth bool
	switch strings.ToLower(value) {
	case "t", "true", "1":
		truth = true
	case "f", "false", "0":
		truth = false
	default:
		return "", false
	}

	switch {
	case dialect == "sqlite" && truth:
		return "1", true
	case dialect == "sqlite":
		return "0", true
	case truth:
		return "TRUE", true
	default:
		return "FALSE", true
	}
}

// adjustDatetime troca o T por espaço e remove o Z final; qualquer outro formato fica como veio.
func adjustDatetime(value string) string {
	m := isoDatetimePattern.FindStringSubmatch(value)
	if m == nil {
		return value
	}
	offset := m[3]
	if offset == "Z" {
		offset = ""
	}
	return m[1] + " " + m[2] + offset
}

// quoteText escreve o texto como literal entre aspas simples, escapando o conteúdo conforme o dialeto.
func quoteText(dialect, s string) string {
	return "'" + escapeLiteral(dialect, s) + "'"
}
