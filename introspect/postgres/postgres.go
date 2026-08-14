// Package postgres reads a PostgreSQL catalog into schema IR. It deliberately
// depends only on database/sql; callers choose and register the driver.
package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/eji/tiqq/schema"
)

func Introspect(ctx context.Context, db *sql.DB, schemaName string) (schema.Schema, error) {
	result := schema.Schema{Name: schemaName}
	tables := map[string]int{}

	rows, err := db.QueryContext(ctx, columnsSQL, schemaName)
	if err != nil {
		return result, fmt.Errorf("introspect columns: %w", err)
	}
	for rows.Next() {
		var tableName string
		var c schema.Column
		var nullable, identity, generated string
		var defaultValue sql.NullString
		if err := rows.Scan(&tableName, &c.Name, &c.DBType, &nullable, &defaultValue, &identity, &generated); err != nil {
			rows.Close()
			return result, fmt.Errorf("scan column: %w", err)
		}
		index, ok := tables[tableName]
		if !ok {
			result.Tables = append(result.Tables, schema.Table{Schema: schemaName, Name: tableName})
			index = len(result.Tables) - 1
			tables[tableName] = index
		}
		c.Nullable, c.Identity, c.Generated = nullable == "YES", identity == "YES", generated != "NEVER"
		if defaultValue.Valid {
			value := defaultValue.String
			c.Default = &value
		}
		result.Tables[index].Columns = append(result.Tables[index].Columns, c)
	}
	if err := rows.Close(); err != nil {
		return result, fmt.Errorf("close columns: %w", err)
	}
	if err := rows.Err(); err != nil {
		return result, fmt.Errorf("iterate columns: %w", err)
	}

	if err := readKeys(ctx, db, schemaName, &result, tables); err != nil {
		return result, err
	}
	if err := readForeignKeys(ctx, db, schemaName, &result, tables); err != nil {
		return result, err
	}
	return result, nil
}

func readKeys(ctx context.Context, db *sql.DB, schemaName string, result *schema.Schema, tables map[string]int) error {
	rows, err := db.QueryContext(ctx, keysSQL, schemaName)
	if err != nil {
		return fmt.Errorf("introspect keys: %w", err)
	}
	defer rows.Close()
	byName := map[string]*schema.Key{}
	keyTypes := map[string]string{}
	for rows.Next() {
		var tableName, name, kind, column string
		if err := rows.Scan(&tableName, &name, &kind, &column); err != nil {
			return fmt.Errorf("scan key: %w", err)
		}
		id := tableName + "\x00" + name
		if byName[id] == nil {
			byName[id] = &schema.Key{Name: name}
			keyTypes[id] = kind
		}
		byName[id].Columns = append(byName[id].Columns, column)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate keys: %w", err)
	}
	for id, key := range byName {
		var tableName string
		for i := range id {
			if id[i] == 0 {
				tableName = id[:i]
				break
			}
		}
		table := &result.Tables[tables[tableName]]
		if keyTypes[id] == "PRIMARY KEY" {
			copy := *key
			table.PrimaryKey = &copy
		} else {
			table.UniqueKeys = append(table.UniqueKeys, *key)
		}
	}
	return nil
}

func readForeignKeys(ctx context.Context, db *sql.DB, schemaName string, result *schema.Schema, tables map[string]int) error {
	rows, err := db.QueryContext(ctx, foreignKeysSQL, schemaName)
	if err != nil {
		return fmt.Errorf("introspect foreign keys: %w", err)
	}
	defer rows.Close()
	byName := map[string]*schema.ForeignKey{}
	owners := map[string]string{}
	for rows.Next() {
		var tableName, name, column, refSchema, refTable, refColumn string
		if err := rows.Scan(&tableName, &name, &column, &refSchema, &refTable, &refColumn); err != nil {
			return fmt.Errorf("scan foreign key: %w", err)
		}
		id := tableName + "\x00" + name
		if byName[id] == nil {
			byName[id] = &schema.ForeignKey{Name: name, ReferencedSchema: refSchema, ReferencedTable: refTable}
			owners[id] = tableName
		}
		byName[id].Columns = append(byName[id].Columns, column)
		byName[id].ReferencedColumns = append(byName[id].ReferencedColumns, refColumn)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate foreign keys: %w", err)
	}
	for id, foreignKey := range byName {
		index := tables[owners[id]]
		result.Tables[index].ForeignKeys = append(result.Tables[index].ForeignKeys, *foreignKey)
	}
	return nil
}

const columnsSQL = `
SELECT table_name, column_name, udt_name, is_nullable, column_default,
       is_identity, is_generated
FROM information_schema.columns
WHERE table_schema = $1
ORDER BY table_name, ordinal_position`

const keysSQL = `
SELECT tc.table_name, tc.constraint_name, tc.constraint_type, kcu.column_name
FROM information_schema.table_constraints tc
JOIN information_schema.key_column_usage kcu
  ON kcu.constraint_schema = tc.constraint_schema
 AND kcu.constraint_name = tc.constraint_name
 AND kcu.table_name = tc.table_name
WHERE tc.table_schema = $1 AND tc.constraint_type IN ('PRIMARY KEY', 'UNIQUE')
ORDER BY tc.table_name, tc.constraint_name, kcu.ordinal_position`

const foreignKeysSQL = `
SELECT fk.table_name, fk.constraint_name, fkc.column_name,
       pk.table_schema, pk.table_name, pkc.column_name
FROM information_schema.table_constraints fk
JOIN information_schema.key_column_usage fkc
  ON fkc.constraint_schema = fk.constraint_schema AND fkc.constraint_name = fk.constraint_name
JOIN information_schema.referential_constraints rc
  ON rc.constraint_schema = fk.constraint_schema AND rc.constraint_name = fk.constraint_name
JOIN information_schema.table_constraints pk
  ON pk.constraint_schema = rc.unique_constraint_schema AND pk.constraint_name = rc.unique_constraint_name
JOIN information_schema.key_column_usage pkc
  ON pkc.constraint_schema = pk.constraint_schema AND pkc.constraint_name = pk.constraint_name
 AND pkc.ordinal_position = fkc.position_in_unique_constraint
WHERE fk.table_schema = $1 AND fk.constraint_type = 'FOREIGN KEY'
ORDER BY fk.table_name, fk.constraint_name, fkc.ordinal_position`
