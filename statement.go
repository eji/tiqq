package tiqq

import (
	"database/sql"
	"fmt"
	"reflect"
)

type Statement struct {
	sql        string
	args       []any
	projection []columnRef
	positions  map[string]int
}

func newStatement(text string, args []any, projection []columnRef) Statement {
	positions := make(map[string]int, len(projection))
	for i, c := range projection {
		positions[c.id] = i
	}
	return Statement{sql: text, args: append([]any(nil), args...), projection: append([]columnRef(nil), projection...), positions: positions}
}

func (s Statement) SQL() string { return s.sql }
func (s Statement) Args() []any { return append([]any(nil), s.args...) }

// Row stores values scanned in projection order. Adapters can construct it
// after database/sql or pgx scanning.
type Row struct {
	statement Statement
	values    []any
}

func NewRow(statement Statement, values ...any) (Row, error) {
	if len(values) != len(statement.projection) {
		return Row{}, fmt.Errorf("tiqq: got %d values for %d projected columns", len(values), len(statement.projection))
	}
	return Row{statement: statement, values: append([]any(nil), values...)}, nil
}

// Get returns the schema-derived type of column. Missing projection membership
// or incompatible driver values panic with a concise diagnostic.
func (r Row) Get[S, V, C any](column Column[S, V, C]) V {
	var zero V
	position, ok := r.statement.positions[column.ref.id]
	if !ok {
		panic("tiqq: column " + column.ref.id + " is not in the projection")
	}
	raw := r.values[position]
	if value, ok := raw.(V); ok {
		return value
	}
	if raw == nil {
		valueType := reflect.TypeOf((*V)(nil)).Elem()
		if valueType.Kind() == reflect.Struct && valueType.PkgPath() == "database/sql" {
			return zero
		}
	}
	panic(fmt.Sprintf("tiqq: column %s: cannot use %T as %T", column.ref.id, raw, zero))
}

// Keep database/sql part of the public nullable contract.
var _ = sql.Null[int32]{}
