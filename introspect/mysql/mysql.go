// Package mysql reads a MySQL information schema into schema IR. It depends
// only on database/sql; the CLI supplies the concrete driver.
package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/eji/tiqq/schema"
)

func Introspect(ctx context.Context, db *sql.DB, databaseName string) (schema.Schema, error) {
	result := schema.Schema{Dialect: schema.MySQL, Name: databaseName}
	tables := map[string]int{}

	rows, err := db.QueryContext(ctx, columnsSQL, databaseName)
	if err != nil {
		return result, fmt.Errorf("introspect MySQL columns: %w", err)
	}
	for rows.Next() {
		var tableName, nullable, extra, columnType string
		var column schema.Column
		var defaultValue, generationExpression sql.NullString
		if err := rows.Scan(
			&tableName, &column.Name, &column.DBType, &nullable, &defaultValue,
			&extra, &generationExpression, &columnType,
		); err != nil {
			rows.Close()
			return result, fmt.Errorf("scan MySQL column: %w", err)
		}
		index, found := tables[tableName]
		if !found {
			result.Tables = append(result.Tables, schema.Table{Schema: databaseName, Name: tableName})
			index = len(result.Tables) - 1
			tables[tableName] = index
		}
		lowerExtra := strings.ToLower(extra)
		column.Nullable = nullable == "YES"
		column.Identity = strings.Contains(lowerExtra, "auto_increment")
		column.Generated = generationExpression.Valid && generationExpression.String != "" || strings.Contains(lowerExtra, "generated")
		column.Unsigned = strings.Contains(strings.ToLower(columnType), " unsigned")
		if defaultValue.Valid {
			value := defaultValue.String
			column.Default = &value
		}
		result.Tables[index].Columns = append(result.Tables[index].Columns, column)
	}
	if err := rows.Close(); err != nil {
		return result, fmt.Errorf("close MySQL columns: %w", err)
	}
	if err := rows.Err(); err != nil {
		return result, fmt.Errorf("iterate MySQL columns: %w", err)
	}
	if err := readKeys(ctx, db, databaseName, &result, tables); err != nil {
		return result, err
	}
	if err := readForeignKeys(ctx, db, databaseName, &result, tables); err != nil {
		return result, err
	}
	return result, nil
}

func readKeys(ctx context.Context, db *sql.DB, databaseName string, result *schema.Schema, tables map[string]int) error {
	rows, err := db.QueryContext(ctx, keysSQL, databaseName)
	if err != nil {
		return fmt.Errorf("introspect MySQL keys: %w", err)
	}
	defer rows.Close()
	byName := map[string]*schema.Key{}
	kinds := map[string]string{}
	owners := map[string]string{}
	for rows.Next() {
		var tableName, name, kind, column string
		if err := rows.Scan(&tableName, &name, &kind, &column); err != nil {
			return fmt.Errorf("scan MySQL key: %w", err)
		}
		id := tableName + "\x00" + name
		if byName[id] == nil {
			byName[id] = &schema.Key{Name: name}
			kinds[id], owners[id] = kind, tableName
		}
		byName[id].Columns = append(byName[id].Columns, column)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate MySQL keys: %w", err)
	}
	for id, key := range byName {
		table := &result.Tables[tables[owners[id]]]
		if kinds[id] == "PRIMARY KEY" {
			copyKey := *key
			table.PrimaryKey = &copyKey
		} else {
			table.UniqueKeys = append(table.UniqueKeys, *key)
		}
	}
	return nil
}

func readForeignKeys(ctx context.Context, db *sql.DB, databaseName string, result *schema.Schema, tables map[string]int) error {
	rows, err := db.QueryContext(ctx, foreignKeysSQL, databaseName)
	if err != nil {
		return fmt.Errorf("introspect MySQL foreign keys: %w", err)
	}
	defer rows.Close()
	byName := map[string]*schema.ForeignKey{}
	owners := map[string]string{}
	for rows.Next() {
		var tableName, name, column, referencedSchema, referencedTable, referencedColumn string
		if err := rows.Scan(&tableName, &name, &column, &referencedSchema, &referencedTable, &referencedColumn); err != nil {
			return fmt.Errorf("scan MySQL foreign key: %w", err)
		}
		id := tableName + "\x00" + name
		if byName[id] == nil {
			byName[id] = &schema.ForeignKey{
				Name: name, ReferencedSchema: referencedSchema, ReferencedTable: referencedTable,
			}
			owners[id] = tableName
		}
		byName[id].Columns = append(byName[id].Columns, column)
		byName[id].ReferencedColumns = append(byName[id].ReferencedColumns, referencedColumn)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate MySQL foreign keys: %w", err)
	}
	for id, foreignKey := range byName {
		index := tables[owners[id]]
		result.Tables[index].ForeignKeys = append(result.Tables[index].ForeignKeys, *foreignKey)
	}
	return nil
}

const columnsSQL = `
SELECT table_name, column_name, data_type, is_nullable, column_default,
       extra, generation_expression, column_type
FROM information_schema.columns
WHERE table_schema = ?
ORDER BY table_name, ordinal_position`

const keysSQL = `
SELECT tc.table_name, tc.constraint_name, tc.constraint_type, kcu.column_name
FROM information_schema.table_constraints tc
JOIN information_schema.key_column_usage kcu
  ON kcu.constraint_schema = tc.constraint_schema
 AND kcu.constraint_name = tc.constraint_name
 AND kcu.table_name = tc.table_name
WHERE tc.table_schema = ? AND tc.constraint_type IN ('PRIMARY KEY', 'UNIQUE')
ORDER BY tc.table_name, tc.constraint_name, kcu.ordinal_position`

const foreignKeysSQL = `
SELECT table_name, constraint_name, column_name,
       referenced_table_schema, referenced_table_name, referenced_column_name
FROM information_schema.key_column_usage
WHERE table_schema = ? AND referenced_table_name IS NOT NULL
ORDER BY table_name, constraint_name, ordinal_position`
