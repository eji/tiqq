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

// Scanner is implemented by *database/sql.Row, *database/sql.Rows, and
// compatible adapters.
type Scanner interface {
	Scan(dest ...any) error
}

// ScanRow scans the current database row in projection order.
func ScanRow(scanner Scanner, statement Statement) (Row, error) {
	values := make([]any, len(statement.projection))
	destinations := make([]any, len(values))
	for index := range values {
		destinations[index] = &values[index]
	}
	if err := scanner.Scan(destinations...); err != nil {
		return Row{}, fmt.Errorf("tiqq: scan row: %w", err)
	}
	return NewRow(statement, values...)
}

func NewRow(statement Statement, values ...any) (Row, error) {
	if len(values) != len(statement.projection) {
		return Row{}, fmt.Errorf("tiqq: got %d values for %d projected columns", len(values), len(statement.projection))
	}
	return Row{statement: statement, values: append([]any(nil), values...)}, nil
}

type resultKey[V any] interface {
	resultRef() columnRef
	resultValue(V)
}

// Get returns the schema-derived result type or a projection/conversion error.
func (r Row) Get[V any, K resultKey[V]](key K) (V, error) {
	column := key.resultRef()
	var zero V
	position, ok := r.statement.positions[column.id]
	if !ok {
		return zero, fmt.Errorf("tiqq: column %s is not in the projection", column.id)
	}
	raw := r.values[position]
	if value, ok := raw.(V); ok {
		return value, nil
	}
	if raw == nil {
		valueType := reflect.TypeOf((*V)(nil)).Elem()
		if valueType.Kind() == reflect.Struct && valueType.PkgPath() == "database/sql" {
			return zero, nil
		}
	}
	wanted := reflect.TypeOf((*V)(nil)).Elem()
	actual := reflect.ValueOf(raw)
	if wanted == reflect.TypeOf(Decimal("")) {
		switch value := raw.(type) {
		case string:
			return any(Decimal(value)).(V), nil
		case []byte:
			return any(Decimal(string(value))).(V), nil
		}
	}
	if actual.IsValid() && actual.Type().ConvertibleTo(wanted) {
		return actual.Convert(wanted).Interface().(V), nil
	}
	if wanted.Kind() == reflect.Struct && wanted.PkgPath() == "database/sql" {
		valueField, found := wanted.FieldByName("V")
		if found && actual.IsValid() && actual.Type().ConvertibleTo(valueField.Type) {
			result := reflect.New(wanted).Elem()
			result.FieldByName("V").Set(actual.Convert(valueField.Type))
			result.FieldByName("Valid").SetBool(true)
			return result.Interface().(V), nil
		}
		if found && valueField.Type == reflect.TypeOf(Decimal("")) {
			result := reflect.New(wanted).Elem()
			switch value := raw.(type) {
			case string:
				result.FieldByName("V").Set(reflect.ValueOf(Decimal(value)))
				result.FieldByName("Valid").SetBool(true)
				return result.Interface().(V), nil
			case []byte:
				result.FieldByName("V").Set(reflect.ValueOf(Decimal(string(value))))
				result.FieldByName("Valid").SetBool(true)
				return result.Interface().(V), nil
			}
		}
	}
	return zero, fmt.Errorf("tiqq: column %s: cannot use %T as %T", column.id, raw, zero)
}

// MustGet returns a typed value and panics if Get fails.
func (r Row) MustGet[V any, K resultKey[V]](key K) V {
	value, err := r.Get(key)
	if err != nil {
		panic(err)
	}
	return value
}

// Keep database/sql part of the public nullable contract.
var _ = sql.Null[int32]{}
