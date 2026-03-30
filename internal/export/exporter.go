package export

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/Felipe-Cavalca/TraceSQL/internal/config"
)

// Run executa a exporta??o de um registro e retorna o SQL gerado.
func Run(ctx context.Context, db *sql.DB, cfg config.Config) (string, error) {
	cfg.Normalize()
	cfg.EnsureDefaults()

	placeholder := placeholderFor(cfg.Driver)
	query := fmt.Sprintf("SELECT * FROM %s WHERE %s = %s LIMIT 1", quoteIdent(cfg.Driver, cfg.Table), quoteIdent(cfg.Driver, cfg.Column), placeholder)

	rows, err := db.QueryContext(ctx, query, cfg.Record)
	if err != nil {
		return "", fmt.Errorf("consulta de origem: %w", err)
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return "", err
	}

	types, err := rows.ColumnTypes()
	if err != nil {
		return "", err
	}

	if !rows.Next() {
		return "", errors.New("nenhum registro encontrado com o valor informado")
	}

	scanned := make([]interface{}, len(cols))
	dests := make([]sql.NullString, len(cols))
	for i := range scanned {
		scanned[i] = &dests[i]
	}

	if err := rows.Scan(scanned...); err != nil {
		return "", err
	}

	insertCols := []string{}
	insertVals := []string{}

	for i, col := range cols {
		if cfg.NewIDs && strings.EqualFold(col, cfg.Column) {
			continue
		}

		insertCols = append(insertCols, quoteIdent(cfg.Driver, col))
		insertVals = append(insertVals, formatValue(dests[i], types[i]))
	}

	if len(insertCols) == 0 {
		return "", errors.New("nenhuma coluna para exportar")
	}

	stmt := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s);\n", quoteIdent(cfg.Driver, cfg.Table), strings.Join(insertCols, ", "), strings.Join(insertVals, ", "))
	return stmt, nil
}

func placeholderFor(driver string) string {
	if strings.HasPrefix(strings.ToLower(driver), "postgres") || strings.ToLower(driver) == "pg" {
		return "$1"
	}
	return "?"
}

func quoteIdent(driver, ident string) string {
	if ident == "" {
		return ident
	}
	clean := strings.ReplaceAll(ident, "`", "")
	clean = strings.ReplaceAll(clean, "\"", "")
	if strings.HasPrefix(strings.ToLower(driver), "postgres") || strings.ToLower(driver) == "pg" {
		return fmt.Sprintf("\"%s\"", clean)
	}
	return fmt.Sprintf("`%s`", clean)
}

func formatValue(v sql.NullString, colType *sql.ColumnType) string {
	if !v.Valid {
		return "NULL"
	}
	typeName := strings.ToLower(colType.DatabaseTypeName())
	switch {
	case strings.Contains(typeName, "int"), strings.Contains(typeName, "float"), strings.Contains(typeName, "double"), strings.Contains(typeName, "dec"):
		return v.String
	case strings.Contains(typeName, "bool"):
		if v.String == "t" || strings.EqualFold(v.String, "true") || v.String == "1" {
			return "TRUE"
		}
		return "FALSE"
	default:
		return "'" + escapeQuotes(v.String) + "'"
	}
}

func escapeQuotes(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}
