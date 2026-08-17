package tiqq

import (
	"database/sql"
)

// Aggregate is a typed aggregate expression and a typed result key.
type Aggregate[S, V, C any] struct {
	ref columnRef
}

func aggregate[S, V, C any](function string, input columnRef) Aggregate[S, V, C] {
	text := function + "(" + renderColumn(input) + ")"
	return Aggregate[S, V, C]{ref: columnRef{
		id: text, qualifier: input.qualifier, name: input.name, sql: text, aggregate: true,
	}}
}

func (a Aggregate[S, V, C]) Eq(value C) Predicate  { return comparison(a.ref, "=", value) }
func (a Aggregate[S, V, C]) Ne(value C) Predicate  { return comparison(a.ref, "<>", value) }
func (a Aggregate[S, V, C]) Gt(value C) Predicate  { return comparison(a.ref, ">", value) }
func (a Aggregate[S, V, C]) Gte(value C) Predicate { return comparison(a.ref, ">=", value) }
func (a Aggregate[S, V, C]) Lt(value C) Predicate  { return comparison(a.ref, "<", value) }
func (a Aggregate[S, V, C]) Lte(value C) Predicate { return comparison(a.ref, "<=", value) }

func (a Aggregate[S, V, C]) selection() columnRef { return a.ref }
func (a Aggregate[S, V, C]) resultRef() columnRef { return a.ref }
func (a Aggregate[S, V, C]) resultValue(V)        {}

// NumericColumn adds PostgreSQL-aware SUM and AVG result types to a column.
type NumericColumn[S, V, C, Sum, Avg any] struct {
	Column[S, V, C]
}

func RequiredNumericColumn[S, C, Sum, Avg any](table, name string) NumericColumn[S, C, C, Sum, Avg] {
	return NumericColumn[S, C, C, Sum, Avg]{Column: RequiredColumn[S, C](table, name)}
}

func NullableNumericColumn[S, C, Sum, Avg any](table, name string) NumericColumn[S, sql.Null[C], C, Sum, Avg] {
	return NumericColumn[S, sql.Null[C], C, Sum, Avg]{Column: NullableColumn[S, C](table, name)}
}

func (c NumericColumn[S, V, C, Sum, Avg]) Sum() Aggregate[S, sql.Null[Sum], Sum] {
	return aggregate[S, sql.Null[Sum], Sum]("SUM", c.ref)
}

func (c NumericColumn[S, V, C, Sum, Avg]) Avg() Aggregate[S, sql.Null[Avg], Avg] {
	return aggregate[S, sql.Null[Avg], Avg]("AVG", c.ref)
}

func AliasNumericColumn[From, To, V, C, Sum, Avg any](column NumericColumn[From, V, C, Sum, Avg], alias string) NumericColumn[To, V, C, Sum, Avg] {
	return NumericColumn[To, V, C, Sum, Avg]{Column: AliasColumn[From, To](column.Column, alias)}
}

func RebindRequiredNumeric[From, To, C, Sum, Avg any](column NumericColumn[From, C, C, Sum, Avg]) NumericColumn[To, C, C, Sum, Avg] {
	return NumericColumn[To, C, C, Sum, Avg]{Column: RebindRequired[From, To](column.Column)}
}

func RebindNullableNumeric[From, To, C, Sum, Avg any](column NumericColumn[From, C, C, Sum, Avg]) NumericColumn[To, sql.Null[C], C, Sum, Avg] {
	return NumericColumn[To, sql.Null[C], C, Sum, Avg]{Column: RebindNullable[From, To](column.Column)}
}

func RebindExistingNullableNumeric[From, To, C, Sum, Avg any](column NumericColumn[From, sql.Null[C], C, Sum, Avg]) NumericColumn[To, sql.Null[C], C, Sum, Avg] {
	return NumericColumn[To, sql.Null[C], C, Sum, Avg]{Column: RebindExistingNullable[From, To](column.Column)}
}
