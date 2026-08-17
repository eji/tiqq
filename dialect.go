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
)

// InsertDialect is the closed set of dialect markers for generated INSERT builders.
type InsertDialect interface{ insertDialect() }

// PostgreSQLMarker ties generated query types to PostgreSQL-only APIs.
type PostgreSQLMarker struct{}

func (PostgreSQLMarker) insertDialect() {}

// MySQLMarker ties generated query types to MySQL-only APIs.
type MySQLMarker struct{}

func (MySQLMarker) insertDialect() {}

func (dialect builtinDialect) dialectRenderer() sqlRenderer {
	switch dialect {
	case PostgreSQL:
		return postgresRenderer{}
	case MySQL:
		return mysqlRenderer{}
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

// SchemaInfo carries code-generation source metadata shared by generated tables.
type SchemaInfo struct{ dialect Dialect }

func NewSchemaInfo(dialect Dialect) SchemaInfo {
	if dialect == nil {
		panic("tiqq: schema dialect must not be nil")
	}
	return SchemaInfo{dialect: dialect}
}

func (schema SchemaInfo) Table(name string) TableRef {
	return TableRef{name: name, dialect: schema.dialect}
}

func rendererFor(table TableRef) (sqlRenderer, error) {
	if table.dialect == nil {
		return nil, fmt.Errorf("tiqq: table %s has no SQL dialect", table.name)
	}
	return table.dialect.dialectRenderer(), nil
}
