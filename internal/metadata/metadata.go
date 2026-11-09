package metadata

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strconv"
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

// ForeignKey representa uma constraint de chave estrangeira. Columns e
// RefColumns têm o mesmo tamanho e estão ordenadas por posição: Columns[i]
// referencia RefColumns[i]. Uma FK simples tem uma única coluna. Name é o nome
// da constraint quando o banco informa (vazio no SQLite e nas FKs inferidas).
type ForeignKey struct {
	Name       string
	Table      string
	Columns    []string
	RefTable   string
	RefColumns []string
}

// IsComposite informa se a FK usa mais de uma coluna.
func (fk ForeignKey) IsComposite() bool {
	return len(fk.Columns) > 1
}

func lessForeignKey(left, right ForeignKey) bool {
	if left.Table != right.Table {
		return left.Table < right.Table
	}
	if l, r := strings.Join(left.Columns, "\x00"), strings.Join(right.Columns, "\x00"); l != r {
		return l < r
	}
	if left.RefTable != right.RefTable {
		return left.RefTable < right.RefTable
	}
	if l, r := strings.Join(left.RefColumns, "\x00"), strings.Join(right.RefColumns, "\x00"); l != r {
		return l < r
	}
	return left.Name < right.Name
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

	sort.SliceStable(catalog.ForeignKeys, func(i, j int) bool {
		return lessForeignKey(catalog.ForeignKeys[i], catalog.ForeignKeys[j])
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
		for _, column := range fk.Columns {
			columnsWithRelations[strings.ToLower(fk.Table)+"."+strings.ToLower(column)] = struct{}{}
		}
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
				Table:      table.Name,
				Columns:    []string{column.Name},
				RefTable:   refTable.Name,
				RefColumns: []string{"id"},
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

	sort.SliceStable(foreignKeys, func(i, j int) bool {
		return lessForeignKey(foreignKeys[i], foreignKeys[j])
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
	return strings.ToLower(fk.Table) + "|" + strings.ToLower(strings.Join(fk.Columns, ",")) + "|" + strings.ToLower(fk.RefTable) + "|" + strings.ToLower(strings.Join(fk.RefColumns, ","))
}

// foreignKeyRow é uma linha de coluna de FK vinda do banco, já na ordem
// posicional dentro de cada constraint.
type foreignKeyRow struct {
	key       string
	name      string
	table     string
	column    string
	refTable  string
	refColumn string
}

// groupForeignKeyRows agrupa as linhas por key (identidade da constraint),
// preservando a ordem de chegada. Colunas de constraints diferentes nunca se
// misturam, mesmo quando compartilham colunas.
func groupForeignKeyRows(rows []foreignKeyRow) []ForeignKey {
	var foreignKeys []ForeignKey
	index := map[string]int{}
	for _, row := range rows {
		i, ok := index[row.key]
		if !ok {
			i = len(foreignKeys)
			index[row.key] = i
			foreignKeys = append(foreignKeys, ForeignKey{Name: row.name, Table: row.table, RefTable: row.refTable})
		}
		foreignKeys[i].Columns = append(foreignKeys[i].Columns, row.column)
		foreignKeys[i].RefColumns = append(foreignKeys[i].RefColumns, row.refColumn)
	}
	return foreignKeys
}

// scanForeignKeyRows lê linhas (constraint_name, tabela, coluna, tabela
// referenciada, coluna referenciada), já ordenadas por constraint e posição,
// e as agrupa por constraint.
func scanForeignKeyRows(rows *sql.Rows) ([]ForeignKey, error) {
	var collected []foreignKeyRow
	for rows.Next() {
		var row foreignKeyRow
		if err := rows.Scan(&row.name, &row.table, &row.column, &row.refTable, &row.refColumn); err != nil {
			return nil, err
		}
		row.key = row.name
		collected = append(collected, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return groupForeignKeyRows(collected), nil
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

	type sqliteFKRow struct {
		id  int
		seq int
		row foreignKeyRow
	}
	var collected []sqliteFKRow
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

		collected = append(collected, sqliteFKRow{id: id, seq: seq, row: foreignKeyRow{
			key:       strconv.Itoa(id),
			table:     table,
			column:    fromCol,
			refTable:  refTable,
			refColumn: toCol,
		}})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// O pragma lista por id e seq, mas a ordem posicional é garantida aqui.
	sort.SliceStable(collected, func(i, j int) bool {
		if collected[i].id != collected[j].id {
			return collected[i].id < collected[j].id
		}
		return collected[i].seq < collected[j].seq
	})
	ordered := make([]foreignKeyRow, 0, len(collected))
	for _, item := range collected {
		ordered = append(ordered, item.row)
	}
	return groupForeignKeyRows(ordered), nil
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
  con.conname,
  cl.relname,
  a.attname,
  rcl.relname AS referenced_table_name,
  ra.attname AS referenced_column_name
FROM pg_catalog.pg_constraint con
JOIN pg_catalog.pg_class cl ON cl.oid = con.conrelid
JOIN pg_catalog.pg_namespace n ON n.oid = cl.relnamespace
JOIN pg_catalog.pg_class rcl ON rcl.oid = con.confrelid
CROSS JOIN LATERAL unnest(con.conkey, con.confkey) WITH ORDINALITY AS k(attnum, refattnum, ord)
JOIN pg_catalog.pg_attribute a ON a.attrelid = con.conrelid AND a.attnum = k.attnum
JOIN pg_catalog.pg_attribute ra ON ra.attrelid = con.confrelid AND ra.attnum = k.refattnum
WHERE con.contype = 'f'
  AND n.nspname = current_schema()
  AND cl.relname = $1
ORDER BY con.conname, k.ord`, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanForeignKeyRows(rows)
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
  constraint_name,
  table_name,
  column_name,
  referenced_table_name,
  referenced_column_name
FROM information_schema.key_column_usage
WHERE table_schema = DATABASE()
  AND table_name = ?
  AND referenced_table_name IS NOT NULL
ORDER BY constraint_name, ordinal_position`, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanForeignKeyRows(rows)
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
	clean := strings.ReplaceAll(ident, `"`, `""`)
	return fmt.Sprintf(`"%s"`, clean)
}
