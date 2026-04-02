package metadata

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
)

type Catalog struct {
	tables      map[string]Table
	ForeignKeys []ForeignKey
}

type Table struct {
	Name    string
	Columns []Column
}

type Column struct {
	Name          string
	Type          string
	Nullable      bool
	PrimaryKey    bool
	AutoIncrement bool
}

type ForeignKey struct {
	Table     string
	Column    string
	RefTable  string
	RefColumn string
}

func Discover(ctx context.Context, db *sql.DB, driver string) (Catalog, error) {
	inspector, err := newInspector(driver)
	if err != nil {
		return Catalog{}, err
	}

	tableNames, err := inspector.listTables(ctx, db)
	if err != nil {
		return Catalog{}, err
	}

	catalog := Catalog{
		tables: make(map[string]Table, len(tableNames)),
	}

	for _, tableName := range tableNames {
		columns, err := inspector.loadColumns(ctx, db, tableName)
		if err != nil {
			return Catalog{}, fmt.Errorf("carregando colunas de %s: %w", tableName, err)
		}

		catalog.tables[strings.ToLower(tableName)] = Table{
			Name:    tableName,
			Columns: columns,
		}

		foreignKeys, err := inspector.loadForeignKeys(ctx, db, tableName)
		if err != nil {
			return Catalog{}, fmt.Errorf("carregando chaves estrangeiras de %s: %w", tableName, err)
		}
		catalog.ForeignKeys = append(catalog.ForeignKeys, foreignKeys...)
	}

	sort.Slice(catalog.ForeignKeys, func(i, j int) bool {
		left := catalog.ForeignKeys[i]
		right := catalog.ForeignKeys[j]
		if left.Table != right.Table {
			return left.Table < right.Table
		}
		if left.Column != right.Column {
			return left.Column < right.Column
		}
		if left.RefTable != right.RefTable {
			return left.RefTable < right.RefTable
		}
		return left.RefColumn < right.RefColumn
	})

	return catalog, nil
}

func (c Catalog) Table(name string) (Table, bool) {
	table, ok := c.tables[strings.ToLower(name)]
	return table, ok
}

func (c Catalog) TableNames() []string {
	names := make([]string, 0, len(c.tables))
	for _, table := range c.tables {
		names = append(names, table.Name)
	}
	sort.Strings(names)
	return names
}

func (c Catalog) FilterTables(keep func(string) bool) Catalog {
	if keep == nil || len(c.tables) == 0 {
		return c
	}

	filteredTables := make(map[string]Table, len(c.tables))
	for key, table := range c.tables {
		if keep(table.Name) {
			filteredTables[key] = table
		}
	}

	filteredForeignKeys := make([]ForeignKey, 0, len(c.ForeignKeys))
	for _, fk := range c.ForeignKeys {
		if _, ok := filteredTables[strings.ToLower(fk.Table)]; !ok {
			continue
		}
		if _, ok := filteredTables[strings.ToLower(fk.RefTable)]; !ok {
			continue
		}
		filteredForeignKeys = append(filteredForeignKeys, fk)
	}

	return Catalog{
		tables:      filteredTables,
		ForeignKeys: filteredForeignKeys,
	}
}

func (c Catalog) WithNameInferredForeignKeys() (Catalog, int) {
	if len(c.tables) == 0 {
		return c, 0
	}

	exactTables := make(map[string]string, len(c.tables))
	aliasCandidates := map[string][]string{}
	for _, table := range c.tables {
		lowerName := strings.ToLower(table.Name)
		exactTables[lowerName] = table.Name
		for _, alias := range tableNameAliases(lowerName) {
			aliasCandidates[alias] = appendUnique(aliasCandidates[alias], table.Name)
		}
	}

	foreignKeys := append([]ForeignKey(nil), c.ForeignKeys...)
	existing := map[string]struct{}{}
	columnsWithRelations := map[string]struct{}{}
	for _, fk := range foreignKeys {
		existing[foreignKeyKey(fk)] = struct{}{}
		columnsWithRelations[strings.ToLower(fk.Table)+"."+strings.ToLower(fk.Column)] = struct{}{}
	}

	added := 0
	for _, tableName := range c.TableNames() {
		table, ok := c.Table(tableName)
		if !ok {
			continue
		}

		for _, column := range table.Columns {
			columnName := strings.ToLower(column.Name)
			if !strings.HasSuffix(columnName, "_id") {
				continue
			}
			if _, exists := columnsWithRelations[strings.ToLower(table.Name)+"."+columnName]; exists {
				continue
			}

			refAlias := strings.TrimSuffix(columnName, "_id")
			refTableName, ok := inferReferenceTable(refAlias, exactTables, aliasCandidates)
			if !ok {
				continue
			}

			refTable, ok := c.Table(refTableName)
			if !ok || !hasPrimaryKeyColumn(refTable, "id") {
				continue
			}

			fk := ForeignKey{
				Table:     table.Name,
				Column:    column.Name,
				RefTable:  refTable.Name,
				RefColumn: "id",
			}
			key := foreignKeyKey(fk)
			if _, exists := existing[key]; exists {
				continue
			}

			existing[key] = struct{}{}
			columnsWithRelations[strings.ToLower(table.Name)+"."+columnName] = struct{}{}
			foreignKeys = append(foreignKeys, fk)
			added++
		}
	}

	if added == 0 {
		return c, 0
	}

	sort.Slice(foreignKeys, func(i, j int) bool {
		left := foreignKeys[i]
		right := foreignKeys[j]
		if left.Table != right.Table {
			return left.Table < right.Table
		}
		if left.Column != right.Column {
			return left.Column < right.Column
		}
		if left.RefTable != right.RefTable {
			return left.RefTable < right.RefTable
		}
		return left.RefColumn < right.RefColumn
	})

	return Catalog{
		tables:      c.tables,
		ForeignKeys: foreignKeys,
	}, added
}

func (t Table) PrimaryKeyColumns() []string {
	var cols []string
	for _, col := range t.Columns {
		if col.PrimaryKey {
			cols = append(cols, col.Name)
		}
	}
	return cols
}

func hasPrimaryKeyColumn(table Table, columnName string) bool {
	for _, column := range table.Columns {
		if strings.EqualFold(column.Name, columnName) && column.PrimaryKey {
			return true
		}
	}
	return false
}

func inferReferenceTable(alias string, exactTables map[string]string, aliasCandidates map[string][]string) (string, bool) {
	if tableName, ok := exactTables[alias]; ok {
		return tableName, true
	}

	candidates := aliasCandidates[alias]
	if len(candidates) != 1 {
		return "", false
	}
	return candidates[0], true
}

func tableNameAliases(name string) []string {
	aliases := []string{name}
	switch {
	case strings.HasSuffix(name, "ies") && len(name) > 3:
		aliases = append(aliases, name[:len(name)-3]+"y")
	case strings.HasSuffix(name, "s") && len(name) > 1:
		aliases = append(aliases, strings.TrimSuffix(name, "s"))
	}
	return aliases
}

func appendUnique(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func foreignKeyKey(fk ForeignKey) string {
	return strings.ToLower(fk.Table) + "|" + strings.ToLower(fk.Column) + "|" + strings.ToLower(fk.RefTable) + "|" + strings.ToLower(fk.RefColumn)
}

type inspector interface {
	listTables(ctx context.Context, db *sql.DB) ([]string, error)
	loadColumns(ctx context.Context, db *sql.DB, table string) ([]Column, error)
	loadForeignKeys(ctx context.Context, db *sql.DB, table string) ([]ForeignKey, error)
}

func newInspector(driver string) (inspector, error) {
	switch normalizeDriver(driver) {
	case "sqlite":
		return sqliteInspector{}, nil
	case "postgres":
		return postgresInspector{}, nil
	case "mysql":
		return mysqlInspector{}, nil
	default:
		return nil, fmt.Errorf("driver não suportado para metadata: %s", driver)
	}
}

func normalizeDriver(driver string) string {
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

type sqliteInspector struct{}

func (sqliteInspector) listTables(ctx context.Context, db *sql.DB) ([]string, error) {
	rows, err := db.QueryContext(ctx, "SELECT name FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%' ORDER BY name")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanSingleStringColumn(rows)
}

func (sqliteInspector) loadColumns(ctx context.Context, db *sql.DB, table string) ([]Column, error) {
	query := fmt.Sprintf("PRAGMA table_info(%s)", quoteSQLiteIdent(table))
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var columns []Column
	for rows.Next() {
		var (
			cid        int
			name       string
			typeName   string
			notNull    int
			defaultVal interface{}
			pk         int
		)
		if err := rows.Scan(&cid, &name, &typeName, &notNull, &defaultVal, &pk); err != nil {
			return nil, err
		}

		columns = append(columns, Column{
			Name:          name,
			Type:          typeName,
			Nullable:      notNull == 0 && pk == 0,
			PrimaryKey:    pk > 0,
			AutoIncrement: pk > 0 && strings.Contains(strings.ToLower(typeName), "int"),
		})
	}

	return columns, rows.Err()
}

func (sqliteInspector) loadForeignKeys(ctx context.Context, db *sql.DB, table string) ([]ForeignKey, error) {
	query := fmt.Sprintf("PRAGMA foreign_key_list(%s)", quoteSQLiteIdent(table))
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var foreignKeys []ForeignKey
	for rows.Next() {
		var (
			id       int
			seq      int
			refTable string
			fromCol  string
			toCol    string
			onUpdate string
			onDelete string
			match    string
		)
		if err := rows.Scan(&id, &seq, &refTable, &fromCol, &toCol, &onUpdate, &onDelete, &match); err != nil {
			return nil, err
		}

		foreignKeys = append(foreignKeys, ForeignKey{
			Table:     table,
			Column:    fromCol,
			RefTable:  refTable,
			RefColumn: toCol,
		})
	}

	return foreignKeys, rows.Err()
}

type postgresInspector struct{}

func (postgresInspector) listTables(ctx context.Context, db *sql.DB) ([]string, error) {
	rows, err := db.QueryContext(ctx, `
SELECT tablename
FROM pg_catalog.pg_tables
WHERE schemaname = current_schema()
ORDER BY tablename`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanSingleStringColumn(rows)
}

func (postgresInspector) loadColumns(ctx context.Context, db *sql.DB, table string) ([]Column, error) {
	rows, err := db.QueryContext(ctx, `
SELECT
  a.attname,
  pg_catalog.format_type(a.atttypid, a.atttypmod),
  NOT a.attnotnull AS is_nullable,
  COALESCE(pg_get_expr(ad.adbin, ad.adrelid), '') AS default_value,
  EXISTS (
    SELECT 1
    FROM pg_index i
    WHERE i.indrelid = c.oid
      AND i.indisprimary
      AND a.attnum = ANY(i.indkey)
  ) AS is_primary_key,
  (a.attidentity IN ('a', 'd') OR COALESCE(pg_get_expr(ad.adbin, ad.adrelid), '') LIKE 'nextval(%') AS is_auto_increment
FROM pg_attribute a
JOIN pg_class c ON c.oid = a.attrelid
JOIN pg_namespace n ON n.oid = c.relnamespace
LEFT JOIN pg_attrdef ad ON ad.adrelid = a.attrelid AND ad.adnum = a.attnum
WHERE c.relname = $1
  AND n.nspname = current_schema()
  AND a.attnum > 0
  AND NOT a.attisdropped
ORDER BY a.attnum`, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var columns []Column
	for rows.Next() {
		var (
			name       string
			typeName   string
			nullable   bool
			defaultVal sql.NullString
			primaryKey bool
			autoInc    bool
		)
		if err := rows.Scan(&name, &typeName, &nullable, &defaultVal, &primaryKey, &autoInc); err != nil {
			return nil, err
		}

		columns = append(columns, Column{
			Name:          name,
			Type:          typeName,
			Nullable:      nullable,
			PrimaryKey:    primaryKey,
			AutoIncrement: autoInc,
		})
	}

	return columns, rows.Err()
}

func (postgresInspector) loadForeignKeys(ctx context.Context, db *sql.DB, table string) ([]ForeignKey, error) {
	rows, err := db.QueryContext(ctx, `
SELECT
  tc.table_name,
  kcu.column_name,
  ccu.table_name AS referenced_table_name,
  ccu.column_name AS referenced_column_name
FROM information_schema.table_constraints tc
JOIN information_schema.key_column_usage kcu
  ON tc.constraint_name = kcu.constraint_name
 AND tc.table_schema = kcu.table_schema
JOIN information_schema.constraint_column_usage ccu
  ON ccu.constraint_name = tc.constraint_name
 AND ccu.table_schema = tc.table_schema
WHERE tc.constraint_type = 'FOREIGN KEY'
  AND tc.table_schema = current_schema()
  AND tc.table_name = $1
ORDER BY kcu.ordinal_position`, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var foreignKeys []ForeignKey
	for rows.Next() {
		var fk ForeignKey
		if err := rows.Scan(&fk.Table, &fk.Column, &fk.RefTable, &fk.RefColumn); err != nil {
			return nil, err
		}
		foreignKeys = append(foreignKeys, fk)
	}

	return foreignKeys, rows.Err()
}

type mysqlInspector struct{}

func (mysqlInspector) listTables(ctx context.Context, db *sql.DB) ([]string, error) {
	rows, err := db.QueryContext(ctx, `
SELECT table_name
FROM information_schema.tables
WHERE table_schema = DATABASE()
  AND table_type = 'BASE TABLE'
ORDER BY table_name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanSingleStringColumn(rows)
}

func (mysqlInspector) loadColumns(ctx context.Context, db *sql.DB, table string) ([]Column, error) {
	rows, err := db.QueryContext(ctx, `
SELECT
  column_name,
  column_type,
  is_nullable,
  column_key,
  extra
FROM information_schema.columns
WHERE table_schema = DATABASE()
  AND table_name = ?
ORDER BY ordinal_position`, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var columns []Column
	for rows.Next() {
		var (
			name       string
			typeName   string
			isNullable string
			columnKey  string
			extra      string
		)
		if err := rows.Scan(&name, &typeName, &isNullable, &columnKey, &extra); err != nil {
			return nil, err
		}

		columns = append(columns, Column{
			Name:          name,
			Type:          typeName,
			Nullable:      strings.EqualFold(isNullable, "YES"),
			PrimaryKey:    strings.EqualFold(columnKey, "PRI"),
			AutoIncrement: strings.Contains(strings.ToLower(extra), "auto_increment"),
		})
	}

	return columns, rows.Err()
}

func (mysqlInspector) loadForeignKeys(ctx context.Context, db *sql.DB, table string) ([]ForeignKey, error) {
	rows, err := db.QueryContext(ctx, `
SELECT
  table_name,
  column_name,
  referenced_table_name,
  referenced_column_name
FROM information_schema.key_column_usage
WHERE table_schema = DATABASE()
  AND table_name = ?
  AND referenced_table_name IS NOT NULL
ORDER BY ordinal_position`, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var foreignKeys []ForeignKey
	for rows.Next() {
		var fk ForeignKey
		if err := rows.Scan(&fk.Table, &fk.Column, &fk.RefTable, &fk.RefColumn); err != nil {
			return nil, err
		}
		foreignKeys = append(foreignKeys, fk)
	}

	return foreignKeys, rows.Err()
}

func scanSingleStringColumn(rows *sql.Rows) ([]string, error) {
	var values []string
	for rows.Next() {
		var value string
		if err := rows.Scan(&value); err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, rows.Err()
}

func quoteSQLiteIdent(ident string) string {
	clean := strings.ReplaceAll(ident, `"`, "")
	return fmt.Sprintf(`"%s"`, clean)
}
