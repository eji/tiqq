package tiqq

import (
	"fmt"
	"strings"
)

// Dialect identifies the SQL renderer used by Build. Dialects are provided by
// tiqq so renderer behavior remains consistent with validation.
type Dialect interface {
	dialectRenderer() sqlRenderer
}

type sqlRenderer interface {
	name() string
	quoteIdentifier(string) string
	placeholder(int) string
}

type builtinDialect uint8

const (
	// PostgreSQL selects PostgreSQL SQL syntax and placeholders.
	PostgreSQL builtinDialect = iota + 1
	// MySQL selects MySQL SQL syntax and placeholders.
	MySQL
	// SQLite selects SQLite SQL syntax and placeholders.
	SQLite
)

// InsertDialect is the closed set of dialect markers for generated INSERT builders.
type InsertDialect interface{ insertDialect() }

// PostgreSQLMarker ties generated query types to PostgreSQL-only APIs.
type PostgreSQLMarker struct{}

func (PostgreSQLMarker) insertDialect() {}

// MySQLMarker ties generated query types to MySQL-only APIs.
type MySQLMarker struct{}

func (MySQLMarker) insertDialect() {}

// SQLiteMarker ties generated query types to SQLite-only APIs.
type SQLiteMarker struct{}

func (SQLiteMarker) insertDialect() {}

func (dialect builtinDialect) dialectRenderer() sqlRenderer {
	switch dialect {
	case PostgreSQL:
		return postgresRenderer{}
	case MySQL:
		return mysqlRenderer{}
	case SQLite:
		return sqliteRenderer{}
	default:
		panic("tiqq: unknown SQL dialect")
	}
}

type postgresRenderer struct{}

func (postgresRenderer) name() string { return "postgresql" }
func (postgresRenderer) quoteIdentifier(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
}
func (postgresRenderer) placeholder(index int) string { return fmt.Sprintf("$%d", index) }

type mysqlRenderer struct{}

func (mysqlRenderer) name() string { return "mysql" }
func (mysqlRenderer) quoteIdentifier(value string) string {
	return "`" + strings.ReplaceAll(value, "`", "``") + "`"
}
func (mysqlRenderer) placeholder(int) string { return "?" }

type sqliteRenderer struct{}

func (sqliteRenderer) name() string { return "sqlite" }
func (sqliteRenderer) quoteIdentifier(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
}
func (sqliteRenderer) placeholder(int) string { return "?" }

// SchemaInfo carries code-generation source metadata shared by generated tables.
type SchemaInfo struct{ dialect Dialect }

// NewSchemaInfo constructs dialect metadata for generated schemas.
func NewSchemaInfo(dialect Dialect) SchemaInfo {
	if dialect == nil {
		panic("tiqq: schema dialect must not be nil")
	}
	return SchemaInfo{dialect: dialect}
}

// Table constructs an unqualified table reference for generated schemas.
func (schema SchemaInfo) Table(name string) TableRef {
	return TableRef{name: name, dialect: schema.dialect}
}

// TableInSchema constructs a schema-qualified table reference. It is primarily
// intended for generated PostgreSQL schemas.
func (schema SchemaInfo) TableInSchema(schemaName, tableName string) TableRef {
	return TableRef{schema: schemaName, name: tableName, dialect: schema.dialect}
}

func rendererFor(table TableRef) (sqlRenderer, error) {
	if table.dialect == nil {
		return nil, fmt.Errorf("tiqq: table %s has no SQL dialect", table.name)
	}
	return table.dialect.dialectRenderer(), nil
}
