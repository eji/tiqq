// Package sqlite reads SQLite schema metadata into schema IR. It depends only
// on database/sql; the CLI supplies the concrete driver.
package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"

	"github.com/eji/tiqq/schema"
)

const schemaName = "main"

func Introspect(ctx context.Context, db *sql.DB) (schema.Schema, error) {
	result := schema.Schema{Dialect: schema.SQLite, Name: schemaName}
	tableNames, err := readTableNames(ctx, db)
	if err != nil {
		return result, err
	}
	for _, tableName := range tableNames {
		table, err := readTable(ctx, db, tableName)
		if err != nil {
			return result, err
		}
		result.Tables = append(result.Tables, table)
	}
	if err := resolveReferencedColumns(&result); err != nil {
		return result, err
	}
	return result, nil
}

func resolveReferencedColumns(result *schema.Schema) error {
	tables := make(map[string]*schema.Table, len(result.Tables))
	for index := range result.Tables {
		tables[result.Tables[index].Name] = &result.Tables[index]
	}
	for tableIndex := range result.Tables {
		for keyIndex := range result.Tables[tableIndex].ForeignKeys {
			key := &result.Tables[tableIndex].ForeignKeys[keyIndex]
			missing := false
			for _, column := range key.ReferencedColumns {
				missing = missing || column == ""
			}
			if !missing {
				continue
			}
			referenced := tables[key.ReferencedTable]
			if referenced == nil || referenced.PrimaryKey == nil || len(referenced.PrimaryKey.Columns) != len(key.Columns) {
				return fmt.Errorf("introspect SQLite foreign key %s: omitted referenced columns do not match a primary key", key.Name)
			}
			key.ReferencedColumns = append([]string(nil), referenced.PrimaryKey.Columns...)
		}
	}
	return nil
}

func readTableNames(ctx context.Context, db *sql.DB) ([]string, error) {
	rows, err := db.QueryContext(ctx, tablesSQL)
	if err != nil {
		return nil, fmt.Errorf("introspect SQLite tables: %w", err)
	}
	defer rows.Close()
	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("scan SQLite table: %w", err)
		}
		names = append(names, name)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate SQLite tables: %w", err)
	}
	return names, nil
}

func readTable(ctx context.Context, db *sql.DB, tableName string) (schema.Table, error) {
	table := schema.Table{Schema: schemaName, Name: tableName}
	rows, err := db.QueryContext(ctx, pragma("table_xinfo", tableName))
	if err != nil {
		return table, fmt.Errorf("introspect SQLite columns for %s: %w", tableName, err)
	}
	var primaryColumns map[int]string = map[int]string{}
	for rows.Next() {
		var cid, notNull, primaryPosition, hidden int
		var column schema.Column
		var defaultValue sql.NullString
		if err := rows.Scan(&cid, &column.Name, &column.DBType, &notNull, &defaultValue, &primaryPosition, &hidden); err != nil {
			rows.Close()
			return table, fmt.Errorf("scan SQLite column for %s: %w", tableName, err)
		}
		column.DBType = strings.ToLower(strings.TrimSpace(column.DBType))
		column.Nullable = notNull == 0 && primaryPosition == 0
		column.Generated = hidden != 0
		if defaultValue.Valid {
			value := defaultValue.String
			column.Default = &value
		}
		if primaryPosition > 0 {
			primaryColumns[primaryPosition] = column.Name
		}
		table.Columns = append(table.Columns, column)
	}
	if err := rows.Close(); err != nil {
		return table, fmt.Errorf("close SQLite columns for %s: %w", tableName, err)
	}
	if err := rows.Err(); err != nil {
		return table, fmt.Errorf("iterate SQLite columns for %s: %w", tableName, err)
	}
	if len(primaryColumns) > 0 {
		columns := orderedColumns(primaryColumns)
		table.PrimaryKey = &schema.Key{Name: tableName + "_pkey", Columns: columns}
		if len(columns) == 1 {
			for index := range table.Columns {
				if table.Columns[index].Name == columns[0] && strings.EqualFold(table.Columns[index].DBType, "integer") {
					table.Columns[index].Identity = true
				}
			}
		}
	}
	if err := readUniqueKeys(ctx, db, &table); err != nil {
		return table, err
	}
	if err := readForeignKeys(ctx, db, &table); err != nil {
		return table, err
	}
	return table, nil
}

func readUniqueKeys(ctx context.Context, db *sql.DB, table *schema.Table) error {
	rows, err := db.QueryContext(ctx, pragma("index_list", table.Name))
	if err != nil {
		return fmt.Errorf("introspect SQLite indexes for %s: %w", table.Name, err)
	}
	type index struct{ name string }
	var indexes []index
	for rows.Next() {
		var sequence, unique, partial int
		var name, origin string
		if err := rows.Scan(&sequence, &name, &unique, &origin, &partial); err != nil {
			rows.Close()
			return fmt.Errorf("scan SQLite index for %s: %w", table.Name, err)
		}
		if unique != 0 && origin != "pk" && partial == 0 {
			indexes = append(indexes, index{name: name})
		}
	}
	sort.Slice(indexes, func(left, right int) bool { return indexes[left].name < indexes[right].name })
	if err := rows.Close(); err != nil {
		return fmt.Errorf("close SQLite indexes for %s: %w", table.Name, err)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate SQLite indexes for %s: %w", table.Name, err)
	}
	for _, candidate := range indexes {
		columns, err := readIndexColumns(ctx, db, table.Name, candidate.name)
		if err != nil {
			return err
		}
		if len(columns) > 0 {
			table.UniqueKeys = append(table.UniqueKeys, schema.Key{Name: candidate.name, Columns: columns})
		}
	}
	return nil
}

func readIndexColumns(ctx context.Context, db *sql.DB, tableName, indexName string) ([]string, error) {
	rows, err := db.QueryContext(ctx, pragma("index_info", indexName))
	if err != nil {
		return nil, fmt.Errorf("introspect SQLite index %s for %s: %w", indexName, tableName, err)
	}
	defer rows.Close()
	columns := map[int]string{}
	valid := true
	for rows.Next() {
		var sequence, cid int
		var name sql.NullString
		if err := rows.Scan(&sequence, &cid, &name); err != nil {
			return nil, fmt.Errorf("scan SQLite index %s for %s: %w", indexName, tableName, err)
		}
		valid = valid && name.Valid
		if name.Valid {
			columns[sequence+1] = name.String
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate SQLite index %s for %s: %w", indexName, tableName, err)
	}
	if !valid {
		return nil, nil
	}
	return orderedColumns(columns), nil
}

func readForeignKeys(ctx context.Context, db *sql.DB, table *schema.Table) error {
	rows, err := db.QueryContext(ctx, pragma("foreign_key_list", table.Name))
	if err != nil {
		return fmt.Errorf("introspect SQLite foreign keys for %s: %w", table.Name, err)
	}
	defer rows.Close()
	keys := map[int]*schema.ForeignKey{}
	for rows.Next() {
		var id, sequence int
		var referencedTable, column string
		var referencedColumn sql.NullString
		var onUpdate, onDelete, match string
		if err := rows.Scan(&id, &sequence, &referencedTable, &column, &referencedColumn, &onUpdate, &onDelete, &match); err != nil {
			return fmt.Errorf("scan SQLite foreign key for %s: %w", table.Name, err)
		}
		if keys[id] == nil {
			keys[id] = &schema.ForeignKey{Name: fmt.Sprintf("%s_fk_%d", table.Name, id), ReferencedSchema: schemaName, ReferencedTable: referencedTable}
		}
		keys[id].Columns = append(keys[id].Columns, column)
		keys[id].ReferencedColumns = append(keys[id].ReferencedColumns, referencedColumn.String)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate SQLite foreign keys for %s: %w", table.Name, err)
	}
	ids := make([]int, 0, len(keys))
	for id := range keys {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	for _, id := range ids {
		table.ForeignKeys = append(table.ForeignKeys, *keys[id])
	}
	return nil
}

func orderedColumns(columns map[int]string) []string {
	positions := make([]int, 0, len(columns))
	for position := range columns {
		positions = append(positions, position)
	}
	sort.Ints(positions)
	result := make([]string, 0, len(columns))
	for _, position := range positions {
		result = append(result, columns[position])
	}
	return result
}

func pragma(name, value string) string {
	return fmt.Sprintf(`PRAGMA %s("%s")`, name, strings.ReplaceAll(value, `"`, `""`))
}

const tablesSQL = `SELECT name FROM sqlite_schema WHERE type = 'table' AND name NOT LIKE 'sqlite_%' ORDER BY name`
