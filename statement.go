package tiqq

import (
	"database/sql"
	"encoding/json/jsontext"
	"fmt"
	"reflect"
	"uuid"
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
func (s Statement) Args() []any {
	if s.args == nil {
		return nil
	}
	arguments := make([]any, len(s.args))
	for index, argument := range s.args {
		switch value := argument.(type) {
		case uuid.UUID:
			arguments[index] = value.String()
		case jsontext.Value:
			arguments[index] = []byte(value)
		default:
			arguments[index] = argument
		}
	}
	return arguments
}

// Row stores values scanned in projection order.
type Row struct {
	statement Statement
	values    []any
}

// Scanner is implemented by database/sql and pgx Row and Rows types.
type Scanner interface {
	Scan(dest ...any) error
}

// ScanRow scans a database/sql or pgx row using the statement's projection.
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
	wanted := reflect.TypeOf((*V)(nil)).Elem()
	if raw == nil {
		if wanted.Kind() == reflect.Struct && wanted.PkgPath() == "database/sql" {
			return zero, nil
		}
	}
	if wanted == reflect.TypeOf(uuid.UUID{}) {
		value, err := uuidValue(raw)
		if err != nil {
			return zero, fmt.Errorf("tiqq: column %s: %w", column.id, err)
		}
		return any(value).(V), nil
	}
	if value, ok := raw.(V); ok {
		return value, nil
	}
	actual := reflect.ValueOf(raw)
	if wanted == reflect.TypeOf(jsontext.Value(nil)) {
		value, err := jsonValue(raw)
		if err != nil {
			return zero, fmt.Errorf("tiqq: column %s: %w", column.id, err)
		}
		return any(value).(V), nil
	}
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
		if found && valueField.Type == reflect.TypeOf(uuid.UUID{}) {
			value, err := uuidValue(raw)
			if err != nil {
				return zero, fmt.Errorf("tiqq: column %s: %w", column.id, err)
			}
			result := reflect.New(wanted).Elem()
			result.FieldByName("V").Set(reflect.ValueOf(value))
			result.FieldByName("Valid").SetBool(true)
			return result.Interface().(V), nil
		}
		if found && valueField.Type == reflect.TypeOf(jsontext.Value(nil)) {
			value, err := jsonValue(raw)
			if err != nil {
				return zero, fmt.Errorf("tiqq: column %s: %w", column.id, err)
			}
			result := reflect.New(wanted).Elem()
			result.FieldByName("V").Set(reflect.ValueOf(value))
			result.FieldByName("Valid").SetBool(true)
			return result.Interface().(V), nil
		}
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

func uuidValue(raw any) (uuid.UUID, error) {
	switch value := raw.(type) {
	case uuid.UUID:
		return value, nil
	case [16]byte:
		return uuid.UUID(value), nil
	case string:
		parsed, err := uuid.Parse(value)
		if err != nil {
			return uuid.UUID{}, fmt.Errorf("invalid UUID: %w", err)
		}
		return parsed, nil
	case []byte:
		if len(value) == 16 {
			var result uuid.UUID
			copy(result[:], value)
			return result, nil
		}
		parsed, err := uuid.Parse(string(value))
		if err != nil {
			return uuid.UUID{}, fmt.Errorf("invalid UUID: %w", err)
		}
		return parsed, nil
	default:
		return uuid.UUID{}, fmt.Errorf("cannot use %T as uuid.UUID", raw)
	}
}

func jsonValue(raw any) (jsontext.Value, error) {
	var value jsontext.Value
	switch raw := raw.(type) {
	case jsontext.Value:
		value = raw.Clone()
	case []byte:
		value = append(jsontext.Value(nil), raw...)
	case string:
		value = jsontext.Value(raw)
	default:
		return nil, fmt.Errorf("cannot use %T as jsontext.Value", raw)
	}
	if !value.IsValid() {
		return nil, fmt.Errorf("invalid JSON")
	}
	return value, nil
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
