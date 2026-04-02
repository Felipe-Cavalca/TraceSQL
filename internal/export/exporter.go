package export

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"github.com/Felipe-Cavalca/TraceSQL/internal/config"
	"github.com/Felipe-Cavalca/TraceSQL/internal/metadata"
)

var (
	decimalTypePattern = regexp.MustCompile(`(?:numeric|decimal)\(([^)]+)\)`)
	varcharTypePattern = regexp.MustCompile(`(?:character varying|varchar)\((\d+)\)`)
	charTypePattern    = regexp.MustCompile(`(?:character|char)\((\d+)\)`)
)

type newIDMapping struct {
	Table    string
	MapTable string
	PK       metadata.Column
}

type traceLogger func(string, ...interface{})

// Run executa a exportação do registro inicial e das suas relações.
func Run(ctx context.Context, db *sql.DB, cfg config.Config) (string, error) {
	cfg.Normalize()
	cfg.EnsureDefaults()
	logf := newTraceLogger(cfg.Log)
	logf("iniciando export: source=%s target=%s table=%s column=%s record=%s new_ids=%t relations_by_name=%t", cfg.Driver, cfg.OutputDriver, cfg.Table, cfg.Column, cfg.Record, cfg.NewIDs, cfg.RelationsByName)

	baseRows, err := queryRowsByValue(ctx, db, cfg.Driver, cfg.Table, cfg.Column, cfg.Record, logf)
	if err != nil {
		return "", fmt.Errorf("consulta de origem: %w", err)
	}
	if len(baseRows) == 0 {
		return "", errors.New("nenhum registro encontrado com o valor informado")
	}
	logf("linhas base encontradas: %d", len(baseRows))

	catalog, err := metadata.Discover(ctx, db, cfg.Driver)
	if err != nil {
		return "", fmt.Errorf("descobrindo metadata: %w", err)
	}
	logf("metadata descoberta: %d tabelas e %d foreign keys", len(catalog.TableNames()), len(catalog.ForeignKeys))
	if cfg.RelationsByName {
		var inferred int
		catalog, inferred = catalog.WithNameInferredForeignKeys()
		logf("relações por nome habilitadas: %d relações inferidas (%d total)", inferred, len(catalog.ForeignKeys))
	}

	if tableMeta, ok := catalog.Table(cfg.Table); ok {
		cfg.Table = tableMeta.Name
	}
	for i := range baseRows {
		baseRows[i].table = cfg.Table
	}

	rowsByTable, err := collectRows(ctx, db, cfg, catalog, baseRows, logf)
	if err != nil {
		return "", err
	}
	if len(rowsByTable) == 0 {
		return "", errors.New("nenhuma linha encontrada para exportação")
	}
	logf("grafo coletado: %s", summarizeRowsByTable(rowsByTable))

	tableOrder := orderedTables(catalog, rowsByTable)
	exportedTables := make(map[string]struct{}, len(tableOrder))
	for _, tableName := range tableOrder {
		exportedTables[strings.ToLower(tableName)] = struct{}{}
	}

	mappings := buildNewIDMappings(catalog, tableOrder, cfg.NewIDs)

	var builder strings.Builder
	for _, tableName := range tableOrder {
		tableMeta, ok := catalog.Table(tableName)
		if !ok {
			return "", fmt.Errorf("metadata ausente para a tabela %s", tableName)
		}
		logf("gerando schema da tabela %s", tableName)
		builder.WriteString(buildCreateTable(cfg.OutputDriver, tableMeta, foreignKeysForTable(catalog, exportedTables, tableName)))
	}

	if cfg.NewIDs {
		for _, tableName := range tableOrder {
			mapping, ok := mappings[strings.ToLower(tableName)]
			if !ok {
				continue
			}
			logf("criando tabela temporária de mapeamento para %s", tableName)
			builder.WriteString(buildMappingTableSetup(cfg.OutputDriver, mapping))
		}
	}

	for _, tableName := range tableOrder {
		rows := orderRowsForInsert(
			catalog,
			tableName,
			rowsByTable[tableName],
			tableForeignKeys(catalog, tableName),
			mappings,
		)

		tableMeta, _ := catalog.Table(tableName)
		logf("gerando inserts da tabela %s (%d linhas)", tableName, len(rows))
		for _, row := range rows {
			stmt, err := buildInsert(cfg, row, tableMeta, tableForeignKeys(catalog, tableName), mappings)
			if err != nil {
				return "", err
			}
			builder.WriteString(stmt)
		}
	}

	if builder.Len() == 0 {
		return "", errors.New("nenhum SQL foi gerado")
	}

	return builder.String(), nil
}

type scannedRow struct {
	table string
	cols  []string
	types []*sql.ColumnType
	dests []sql.NullString
	index map[string]int
}

func collectRows(ctx context.Context, db *sql.DB, cfg config.Config, catalog metadata.Catalog, baseRows []scannedRow, logf traceLogger) (map[string][]scannedRow, error) {
	queue := append([]scannedRow(nil), baseRows...)
	rowsByTable := map[string][]scannedRow{}
	seen := map[string]struct{}{}

	for len(queue) > 0 {
		row := queue[0]
		queue = queue[1:]

		tableMeta, ok := catalog.Table(row.table)
		if ok {
			row.table = tableMeta.Name
		}

		identity := rowIdentity(tableMeta, row)
		if _, exists := seen[identity]; exists {
			continue
		}
		seen[identity] = struct{}{}
		rowsByTable[row.table] = append(rowsByTable[row.table], row)

		relatedRows, err := relatedRowsForRow(ctx, db, cfg.Driver, catalog, row, logf)
		if err != nil {
			return nil, err
		}
		queue = append(queue, relatedRows...)
	}

	return rowsByTable, nil
}

func relatedRowsForRow(ctx context.Context, db *sql.DB, sourceDriver string, catalog metadata.Catalog, row scannedRow, logf traceLogger) ([]scannedRow, error) {
	var related []scannedRow

	for _, fk := range catalog.ForeignKeys {
		switch {
		case strings.EqualFold(fk.Table, row.table):
			value, ok := valueForColumn(row, fk.Column)
			if !ok || !value.Valid {
				continue
			}

			parentRows, err := queryRowsByValue(ctx, db, sourceDriver, fk.RefTable, fk.RefColumn, value.String, logf)
			if err != nil {
				return nil, fmt.Errorf("consultando pai %s.%s -> %s.%s: %w", fk.Table, fk.Column, fk.RefTable, fk.RefColumn, err)
			}
			related = append(related, parentRows...)

		case strings.EqualFold(fk.RefTable, row.table):
			value, ok := valueForColumn(row, fk.RefColumn)
			if !ok || !value.Valid {
				continue
			}

			childRows, err := queryRowsByValue(ctx, db, sourceDriver, fk.Table, fk.Column, value.String, logf)
			if err != nil {
				return nil, fmt.Errorf("consultando filho %s.%s -> %s.%s: %w", fk.Table, fk.Column, fk.RefTable, fk.RefColumn, err)
			}
			related = append(related, childRows...)
		}
	}

	return related, nil
}

func queryRowsByValue(ctx context.Context, db *sql.DB, driver, table, column, value string, logf traceLogger) ([]scannedRow, error) {
	logf("consultando %s onde %s = %s", table, column, value)
	query := fmt.Sprintf(
		"SELECT * FROM %s WHERE %s = %s",
		quoteIdent(driver, table),
		quoteIdent(driver, column),
		placeholderFor(driver),
	)

	rows, err := scanQuery(ctx, db, query, value)
	if err != nil {
		return nil, err
	}
	for i := range rows {
		rows[i].table = table
	}
	logf("consulta retornou %d linhas para %s", len(rows), table)
	return rows, nil
}

func scanQuery(ctx context.Context, db *sql.DB, query string, args ...interface{}) ([]scannedRow, error) {
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	types, err := rows.ColumnTypes()
	if err != nil {
		return nil, err
	}

	var out []scannedRow
	for rows.Next() {
		scanned := make([]interface{}, len(cols))
		dests := make([]sql.NullString, len(cols))
		index := make(map[string]int, len(cols))
		for i := range scanned {
			scanned[i] = &dests[i]
			index[strings.ToLower(cols[i])] = i
		}
		if err := rows.Scan(scanned...); err != nil {
			return nil, err
		}
		out = append(out, scannedRow{
			cols:  cols,
			types: types,
			dests: dests,
			index: index,
		})
	}

	return out, rows.Err()
}

func buildCreateTable(outputDriver string, table metadata.Table, foreignKeys []metadata.ForeignKey) string {
	pkColumns := table.PrimaryKeyColumns()
	inlineAutoPrimaryKey := len(pkColumns) == 1

	definitions := make([]string, 0, len(table.Columns)+len(foreignKeys)+1)
	for _, column := range table.Columns {
		columnType := mapColumnType(outputDriver, column.Type)
		identifier := quoteIdent(outputDriver, column.Name)

		if normalizeDialect(outputDriver) == "sqlite" && inlineAutoPrimaryKey && column.PrimaryKey && column.AutoIncrement {
			definitions = append(definitions, fmt.Sprintf("%s INTEGER PRIMARY KEY AUTOINCREMENT", identifier))
			continue
		}

		var parts []string
		parts = append(parts, identifier, columnType)
		if column.AutoIncrement && isIntegerType(columnType) {
			switch normalizeDialect(outputDriver) {
			case "postgres":
				parts = append(parts, "GENERATED BY DEFAULT AS IDENTITY")
			case "mysql":
				parts = append(parts, "AUTO_INCREMENT")
			}
		}
		if column.PrimaryKey || !column.Nullable {
			parts = append(parts, "NOT NULL")
		}

		definitions = append(definitions, strings.Join(parts, " "))
	}

	if len(pkColumns) > 0 && !(normalizeDialect(outputDriver) == "sqlite" && inlineAutoPrimaryKey && hasSingleAutoPrimaryKey(table)) {
		definitions = append(definitions, fmt.Sprintf("PRIMARY KEY (%s)", joinQuoted(outputDriver, pkColumns)))
	}

	for _, fk := range foreignKeys {
		definitions = append(definitions, fmt.Sprintf(
			"FOREIGN KEY (%s) REFERENCES %s (%s)",
			quoteIdent(outputDriver, fk.Column),
			quoteIdent(outputDriver, fk.RefTable),
			quoteIdent(outputDriver, fk.RefColumn),
		))
	}

	return fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (\n  %s\n);\n", quoteIdent(outputDriver, table.Name), strings.Join(definitions, ",\n  "))
}

func hasSingleAutoPrimaryKey(table metadata.Table) bool {
	_, ok := singleAutoIncrementPrimaryKey(table)
	return ok
}

func buildInsert(cfg config.Config, row scannedRow, tableMeta metadata.Table, tableForeignKeys []metadata.ForeignKey, mappings map[string]newIDMapping) (string, error) {
	table := row.table
	if table == "" {
		table = cfg.Table
	}

	mapping, mappedTable := mappings[strings.ToLower(table)]
	insertCols := []string{}
	insertVals := []string{}

	for i, col := range row.cols {
		if cfg.NewIDs && mappedTable && strings.EqualFold(col, mapping.PK.Name) {
			continue
		}

		insertCols = append(insertCols, quoteIdent(cfg.OutputDriver, col))
		insertVals = append(insertVals, buildValueExpression(cfg.OutputDriver, row, i, tableForeignKeys, mappings))
	}

	if len(insertCols) == 0 {
		return "", errors.New("nenhuma coluna para exportar")
	}

	if cfg.NewIDs && mappedTable {
		oldPKValue, ok := valueForColumn(row, mapping.PK.Name)
		if !ok || !oldPKValue.Valid {
			return "", fmt.Errorf("não foi possível localizar o valor antigo da PK %s.%s", table, mapping.PK.Name)
		}
		pkType := row.types[row.index[strings.ToLower(mapping.PK.Name)]]
		return buildInsertWithMapping(cfg.OutputDriver, table, insertCols, insertVals, mapping, formatValue(oldPKValue, pkType)), nil
	}

	return fmt.Sprintf(
		"INSERT INTO %s (%s) VALUES (%s);\n",
		quoteIdent(cfg.OutputDriver, table),
		strings.Join(insertCols, ", "),
		strings.Join(insertVals, ", "),
	), nil
}

func buildValueExpression(outputDriver string, row scannedRow, columnIndex int, tableForeignKeys []metadata.ForeignKey, mappings map[string]newIDMapping) string {
	columnName := row.cols[columnIndex]
	for _, fk := range tableForeignKeys {
		if !strings.EqualFold(fk.Column, columnName) {
			continue
		}

		mapping, ok := mappings[strings.ToLower(fk.RefTable)]
		if !ok || !strings.EqualFold(fk.RefColumn, mapping.PK.Name) {
			break
		}

		return buildMappedReferenceExpression(outputDriver, mapping, row.dests[columnIndex], row.types[columnIndex])
	}

	return formatValue(row.dests[columnIndex], row.types[columnIndex])
}

func buildInsertWithMapping(outputDriver, table string, insertCols, insertVals []string, mapping newIDMapping, oldPKExpr string) string {
	insertTarget := quoteIdent(outputDriver, table)
	insertColumns := strings.Join(insertCols, ", ")
	insertValues := strings.Join(insertVals, ", ")
	mapTarget := quoteIdent(outputDriver, mapping.MapTable)
	oldIDColumn := quoteIdent(outputDriver, "old_id")
	newIDColumn := quoteIdent(outputDriver, "new_id")
	pkColumn := quoteIdent(outputDriver, mapping.PK.Name)

	switch normalizeDialect(outputDriver) {
	case "postgres":
		return fmt.Sprintf(
			"WITH inserted AS (\n  INSERT INTO %s (%s) VALUES (%s) RETURNING %s\n)\nINSERT INTO %s (%s, %s)\nSELECT %s, %s FROM inserted;\n",
			insertTarget,
			insertColumns,
			insertValues,
			pkColumn,
			mapTarget,
			oldIDColumn,
			newIDColumn,
			oldPKExpr,
			pkColumn,
		)
	case "mysql":
		return fmt.Sprintf(
			"INSERT INTO %s (%s) VALUES (%s);\nINSERT INTO %s (%s, %s) VALUES (%s, LAST_INSERT_ID());\n",
			insertTarget,
			insertColumns,
			insertValues,
			mapTarget,
			oldIDColumn,
			newIDColumn,
			oldPKExpr,
		)
	default:
		return fmt.Sprintf(
			"INSERT INTO %s (%s) VALUES (%s);\nINSERT INTO %s (%s, %s) VALUES (%s, last_insert_rowid());\n",
			insertTarget,
			insertColumns,
			insertValues,
			mapTarget,
			oldIDColumn,
			newIDColumn,
			oldPKExpr,
		)
	}
}

func buildMappedReferenceExpression(outputDriver string, mapping newIDMapping, oldValue sql.NullString, colType *sql.ColumnType) string {
	if !oldValue.Valid {
		return "NULL"
	}

	return fmt.Sprintf(
		"(SELECT %s FROM %s WHERE %s = %s)",
		quoteIdent(outputDriver, "new_id"),
		quoteIdent(outputDriver, mapping.MapTable),
		quoteIdent(outputDriver, "old_id"),
		formatValue(oldValue, colType),
	)
}

func buildNewIDMappings(catalog metadata.Catalog, tableOrder []string, enabled bool) map[string]newIDMapping {
	if !enabled {
		return nil
	}

	mappings := map[string]newIDMapping{}
	for _, tableName := range tableOrder {
		tableMeta, ok := catalog.Table(tableName)
		if !ok {
			continue
		}

		pk, ok := singleAutoIncrementPrimaryKey(tableMeta)
		if !ok {
			continue
		}

		mappings[strings.ToLower(tableName)] = newIDMapping{
			Table:    tableMeta.Name,
			MapTable: mappingTableName(tableMeta.Name),
			PK:       pk,
		}
	}
	return mappings
}

func singleAutoIncrementPrimaryKey(table metadata.Table) (metadata.Column, bool) {
	pkColumns := table.PrimaryKeyColumns()
	if len(pkColumns) != 1 {
		return metadata.Column{}, false
	}

	for _, column := range table.Columns {
		if column.PrimaryKey && column.AutoIncrement {
			return column, true
		}
	}

	return metadata.Column{}, false
}

func buildMappingTableSetup(outputDriver string, mapping newIDMapping) string {
	tableName := quoteIdent(outputDriver, mapping.MapTable)
	idType := "BIGINT"
	if normalizeDialect(outputDriver) == "sqlite" {
		idType = "INTEGER"
	}

	switch normalizeDialect(outputDriver) {
	case "mysql":
		return fmt.Sprintf(
			"DROP TEMPORARY TABLE IF EXISTS %s;\nCREATE TEMPORARY TABLE %s (%s %s NOT NULL PRIMARY KEY, %s %s NOT NULL);\n",
			tableName,
			tableName,
			quoteIdent(outputDriver, "old_id"),
			idType,
			quoteIdent(outputDriver, "new_id"),
			idType,
		)
	default:
		return fmt.Sprintf(
			"DROP TABLE IF EXISTS %s;\nCREATE TEMP TABLE %s (%s %s NOT NULL PRIMARY KEY, %s %s NOT NULL);\n",
			tableName,
			tableName,
			quoteIdent(outputDriver, "old_id"),
			idType,
			quoteIdent(outputDriver, "new_id"),
			idType,
		)
	}
}

func mappingTableName(table string) string {
	var builder strings.Builder
	builder.WriteString("__tracesql_map_")
	for _, r := range strings.ToLower(table) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			builder.WriteRune(r)
			continue
		}
		builder.WriteRune('_')
	}
	return builder.String()
}

func foreignKeysForTable(catalog metadata.Catalog, exportedTables map[string]struct{}, table string) []metadata.ForeignKey {
	var filtered []metadata.ForeignKey
	for _, fk := range catalog.ForeignKeys {
		if !strings.EqualFold(fk.Table, table) {
			continue
		}
		if _, ok := exportedTables[strings.ToLower(fk.RefTable)]; !ok {
			continue
		}
		filtered = append(filtered, fk)
	}
	return filtered
}

func tableForeignKeys(catalog metadata.Catalog, table string) []metadata.ForeignKey {
	var filtered []metadata.ForeignKey
	for _, fk := range catalog.ForeignKeys {
		if strings.EqualFold(fk.Table, table) {
			filtered = append(filtered, fk)
		}
	}
	return filtered
}

func orderRowsForInsert(catalog metadata.Catalog, tableName string, rows []scannedRow, tableForeignKeys []metadata.ForeignKey, mappings map[string]newIDMapping) []scannedRow {
	ordered := append([]scannedRow(nil), rows...)
	tableMeta, _ := catalog.Table(tableName)
	sort.Slice(ordered, func(i, j int) bool {
		return rowIdentity(tableMeta, ordered[i]) < rowIdentity(tableMeta, ordered[j])
	})

	mapping, ok := mappings[strings.ToLower(tableName)]
	if !ok {
		return ordered
	}

	var selfReferences []metadata.ForeignKey
	for _, fk := range tableForeignKeys {
		if strings.EqualFold(fk.RefTable, tableName) && strings.EqualFold(fk.RefColumn, mapping.PK.Name) {
			selfReferences = append(selfReferences, fk)
		}
	}
	if len(selfReferences) == 0 {
		return ordered
	}

	pending := append([]scannedRow(nil), ordered...)
	var result []scannedRow

	for len(pending) > 0 {
		pendingIDs := map[string]struct{}{}
		for _, row := range pending {
			value, ok := valueForColumn(row, mapping.PK.Name)
			if ok && value.Valid {
				pendingIDs[value.String] = struct{}{}
			}
		}

		progress := false
		nextPending := make([]scannedRow, 0, len(pending))
		for _, row := range pending {
			ready := true
			for _, fk := range selfReferences {
				value, ok := valueForColumn(row, fk.Column)
				if !ok || !value.Valid {
					continue
				}
				if _, blocked := pendingIDs[value.String]; blocked {
					currentID, hasCurrentID := valueForColumn(row, mapping.PK.Name)
					if !hasCurrentID || !currentID.Valid || currentID.String != value.String {
						ready = false
						break
					}
				}
			}

			if ready {
				result = append(result, row)
				progress = true
				continue
			}
			nextPending = append(nextPending, row)
		}

		if !progress {
			return ordered
		}
		pending = nextPending
	}

	return result
}

func orderedTables(catalog metadata.Catalog, rowsByTable map[string][]scannedRow) []string {
	exportedTables := make(map[string]string, len(rowsByTable))
	for tableName := range rowsByTable {
		exportedTables[strings.ToLower(tableName)] = tableName
	}

	adjacency := map[string][]string{}
	indegree := map[string]int{}
	for _, tableName := range catalog.TableNames() {
		if _, ok := exportedTables[strings.ToLower(tableName)]; ok {
			indegree[tableName] = 0
		}
	}

	seenEdges := map[string]struct{}{}
	for _, fk := range catalog.ForeignKeys {
		parentName, parentOK := exportedTables[strings.ToLower(fk.RefTable)]
		childName, childOK := exportedTables[strings.ToLower(fk.Table)]
		if !parentOK || !childOK {
			continue
		}

		edgeKey := strings.ToLower(parentName) + "->" + strings.ToLower(childName)
		if _, exists := seenEdges[edgeKey]; exists {
			continue
		}
		seenEdges[edgeKey] = struct{}{}
		adjacency[parentName] = append(adjacency[parentName], childName)
		indegree[childName]++
	}

	var ready []string
	for tableName := range indegree {
		if indegree[tableName] == 0 {
			ready = append(ready, tableName)
		}
	}
	sort.Strings(ready)

	var ordered []string
	for len(ready) > 0 {
		current := ready[0]
		ready = ready[1:]
		ordered = append(ordered, current)

		children := adjacency[current]
		sort.Strings(children)
		for _, child := range children {
			indegree[child]--
			if indegree[child] == 0 {
				ready = append(ready, child)
				sort.Strings(ready)
			}
		}
	}

	if len(ordered) == len(exportedTables) {
		return ordered
	}

	var remaining []string
	for _, tableName := range catalog.TableNames() {
		if _, ok := exportedTables[strings.ToLower(tableName)]; !ok {
			continue
		}
		if !containsString(ordered, tableName) {
			remaining = append(remaining, tableName)
		}
	}
	sort.Strings(remaining)
	return append(ordered, remaining...)
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func rowIdentity(table metadata.Table, row scannedRow) string {
	keyParts := []string{strings.ToLower(row.table)}
	pkColumns := table.PrimaryKeyColumns()
	if len(pkColumns) > 0 {
		for _, column := range pkColumns {
			value, ok := valueForColumn(row, column)
			if !ok {
				pkColumns = nil
				break
			}
			keyParts = append(keyParts, strings.ToLower(column), nullableValueString(value))
		}
		if len(pkColumns) > 0 {
			return strings.Join(keyParts, "|")
		}
	}

	keyParts = keyParts[:1]
	for i, column := range row.cols {
		keyParts = append(keyParts, strings.ToLower(column), nullableValueString(row.dests[i]))
	}
	return strings.Join(keyParts, "|")
}

func valueForColumn(row scannedRow, column string) (sql.NullString, bool) {
	position, ok := row.index[strings.ToLower(column)]
	if !ok {
		return sql.NullString{}, false
	}
	return row.dests[position], true
}

func nullableValueString(value sql.NullString) string {
	if !value.Valid {
		return "NULL"
	}
	return value.String
}

func placeholderFor(driver string) string {
	if normalizeDialect(driver) == "postgres" {
		return "$1"
	}
	return "?"
}

func quoteIdent(driver, ident string) string {
	if ident == "" {
		return ident
	}

	clean := strings.ReplaceAll(ident, "`", "")
	clean = strings.ReplaceAll(clean, `"`, "")
	if normalizeDialect(driver) == "postgres" {
		return fmt.Sprintf(`"%s"`, clean)
	}
	return fmt.Sprintf("`%s`", clean)
}

func joinQuoted(driver string, identifiers []string) string {
	quoted := make([]string, 0, len(identifiers))
	for _, identifier := range identifiers {
		quoted = append(quoted, quoteIdent(driver, identifier))
	}
	return strings.Join(quoted, ", ")
}

func normalizeDialect(driver string) string {
	switch strings.ToLower(driver) {
	case "postgres", "postgresql", "pg":
		return "postgres"
	case "mysql":
		return "mysql"
	case "sqlite", "sqlite3":
		return "sqlite"
	default:
		return strings.ToLower(driver)
	}
}

func mapColumnType(targetDriver, rawType string) string {
	target := normalizeDialect(targetDriver)
	lower := strings.ToLower(strings.TrimSpace(rawType))

	switch {
	case lower == "":
		return "TEXT"
	case strings.Contains(lower, "bigserial"), strings.Contains(lower, "bigint"), strings.Contains(lower, "int8"):
		return mapIntegerType(target, "big")
	case strings.Contains(lower, "smallserial"), strings.Contains(lower, "smallint"), strings.Contains(lower, "int2"):
		return mapIntegerType(target, "small")
	case strings.Contains(lower, "serial"), strings.Contains(lower, "integer"), strings.Contains(lower, "int"), strings.Contains(lower, "mediumint"):
		if strings.Contains(lower, "tinyint(1)") {
			return "BOOLEAN"
		}
		return mapIntegerType(target, "default")
	case strings.Contains(lower, "bool"):
		return "BOOLEAN"
	case decimalTypePattern.MatchString(lower):
		match := decimalTypePattern.FindStringSubmatch(lower)
		return mapDecimalType(target, match[1])
	case strings.Contains(lower, "numeric"), strings.Contains(lower, "decimal"):
		return mapDecimalType(target, "")
	case strings.Contains(lower, "double"), strings.Contains(lower, "float8"):
		return mapFloatType(target, true)
	case strings.Contains(lower, "real"), strings.Contains(lower, "float4"):
		return mapFloatType(target, false)
	case strings.Contains(lower, "float"):
		return mapFloatType(target, true)
	case varcharTypePattern.MatchString(lower):
		match := varcharTypePattern.FindStringSubmatch(lower)
		return fmt.Sprintf("VARCHAR(%s)", match[1])
	case charTypePattern.MatchString(lower):
		match := charTypePattern.FindStringSubmatch(lower)
		return fmt.Sprintf("CHAR(%s)", match[1])
	case strings.Contains(lower, "text"), strings.Contains(lower, "clob"):
		return "TEXT"
	case strings.Contains(lower, "json"):
		if target == "sqlite" {
			return "TEXT"
		}
		return "JSON"
	case strings.Contains(lower, "uuid"):
		switch target {
		case "postgres":
			return "UUID"
		case "mysql":
			return "CHAR(36)"
		default:
			return "TEXT"
		}
	case strings.Contains(lower, "timestamp"), strings.Contains(lower, "datetime"):
		switch target {
		case "postgres":
			return "TIMESTAMP"
		case "mysql":
			return "DATETIME"
		default:
			return "DATETIME"
		}
	case strings.HasPrefix(lower, "date"):
		return "DATE"
	case strings.HasPrefix(lower, "time"):
		return "TIME"
	case strings.Contains(lower, "blob"), strings.Contains(lower, "binary"), strings.Contains(lower, "bytea"):
		if target == "postgres" {
			return "BYTEA"
		}
		return "BLOB"
	default:
		return "TEXT"
	}
}

func mapIntegerType(target, size string) string {
	switch target {
	case "postgres":
		switch size {
		case "big":
			return "BIGINT"
		case "small":
			return "SMALLINT"
		default:
			return "INTEGER"
		}
	case "mysql":
		switch size {
		case "big":
			return "BIGINT"
		case "small":
			return "SMALLINT"
		default:
			return "INT"
		}
	default:
		return "INTEGER"
	}
}

func mapDecimalType(target, spec string) string {
	base := "DECIMAL"
	if target == "postgres" {
		base = "NUMERIC"
	}
	if spec == "" {
		return base
	}
	return fmt.Sprintf("%s(%s)", base, strings.ReplaceAll(spec, " ", ""))
}

func mapFloatType(target string, wide bool) string {
	switch target {
	case "postgres":
		if wide {
			return "DOUBLE PRECISION"
		}
		return "REAL"
	case "mysql":
		if wide {
			return "DOUBLE"
		}
		return "FLOAT"
	default:
		return "REAL"
	}
}

func isIntegerType(columnType string) bool {
	lower := strings.ToLower(columnType)
	return strings.Contains(lower, "int")
}

func formatValue(v sql.NullString, colType *sql.ColumnType) string {
	if !v.Valid {
		return "NULL"
	}

	typeName := strings.ToLower(colType.DatabaseTypeName())
	switch {
	case strings.Contains(typeName, "int"), strings.Contains(typeName, "float"), strings.Contains(typeName, "double"), strings.Contains(typeName, "dec"), strings.Contains(typeName, "numeric"), strings.Contains(typeName, "real"):
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

func newTraceLogger(enabled bool) traceLogger {
	if !enabled {
		return func(string, ...interface{}) {}
	}
	return func(format string, args ...interface{}) {
		fmt.Fprintf(os.Stderr, "[tracesql] "+format+"\n", args...)
	}
}

func summarizeRowsByTable(rowsByTable map[string][]scannedRow) string {
	if len(rowsByTable) == 0 {
		return "nenhuma tabela"
	}

	tables := make([]string, 0, len(rowsByTable))
	for tableName := range rowsByTable {
		tables = append(tables, tableName)
	}
	sort.Strings(tables)

	parts := make([]string, 0, len(tables))
	for _, tableName := range tables {
		parts = append(parts, fmt.Sprintf("%s=%d", tableName, len(rowsByTable[tableName])))
	}
	return strings.Join(parts, ", ")
}
