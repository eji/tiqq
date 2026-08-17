// Package schema defines the database-derived intermediate representation used
// by introspectors and generators.
package schema

type Schema struct {
	Dialect Dialect
	Name    string
	Tables  []Table
}

type Dialect string

const PostgreSQL Dialect = "postgresql"

type Table struct {
	Schema      string
	Name        string
	Columns     []Column
	PrimaryKey  *Key
	UniqueKeys  []Key
	ForeignKeys []ForeignKey
}

type Column struct {
	Name      string
	DBType    string
	Nullable  bool
	Default   *string
	Generated bool
	Identity  bool
}

type Key struct {
	Name    string
	Columns []string
}

type ForeignKey struct {
	Name              string
	Columns           []string
	ReferencedSchema  string
	ReferencedTable   string
	ReferencedColumns []string
}
