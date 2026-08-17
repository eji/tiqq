package tiqq

import "database/sql"

// Column is a typed SQL column. V is the value returned by a row and C is the
// value accepted by comparisons. They differ for nullable outer-join columns.
type Column[S, V, C any] struct {
	ref columnRef
}

type columnRef struct {
	id        string
	qualifier string
	name      string
	aggregate bool
	function  string
	input     *columnRef
}

// Assignment is a type-safe UPDATE assignment for scope S.
type Assignment[S any] struct {
	column   columnRef
	value    any
	excluded bool
}

func (assignment Assignment[S]) updateValue(S) Assignment[S] { return assignment }

// UpdateValue is the closed set of typed values accepted by an update action.
type UpdateValue[S any] interface {
	updateValue(S) Assignment[S]
}

// ConflictColumn is a typed conflict-target column for table scope S.
type ConflictColumn[S any] interface {
	conflictColumn(S) columnRef
}

// InsertValue is a type-safe column/value pair for an INSERT statement.
type InsertValue[S any] struct {
	column columnRef
	value  any
}

// AliasColumn rebinds a column to an explicitly named table instance. The
// alias becomes part of both its SQL qualifier and projection identity.
func AliasColumn[From, To, V, C any](column Column[From, V, C], alias string) Column[To, V, C] {
	if alias == "" {
		panic("tiqq: table alias must not be empty")
	}
	return Column[To, V, C]{ref: columnRef{
		id:        alias + "." + column.ref.name,
		qualifier: alias,
		name:      column.ref.name,
	}}
}

// RequireDistinctAliases rejects an ambiguous self join. Intended for
// generated alias join methods.
func RequireDistinctAliases(left, right string) {
	if left == right {
		panic("tiqq: self join aliases must be distinct")
	}
}

// RequiredColumn constructs a non-nullable column. It is primarily intended
// for generated code.
func RequiredColumn[S, T any](table, name string) Column[S, T, T] {
	return Column[S, T, T]{ref: columnRef{id: table + "." + name, qualifier: table, name: name}}
}

// NullableColumn constructs a schema-nullable column for generated code.
func NullableColumn[S, T any](table, name string) Column[S, sql.Null[T], T] {
	return Column[S, sql.Null[T], T]{ref: columnRef{id: table + "." + name, qualifier: table, name: name}}
}

// RebindRequired changes a column's predicate scope. Intended for generated joins.
func RebindRequired[From, To, T any](c Column[From, T, T]) Column[To, T, T] {
	return Column[To, T, T]{ref: c.ref}
}

// RebindNullable changes scope and makes an outer-joined column nullable.
func RebindNullable[From, To, T any](c Column[From, T, T]) Column[To, sql.Null[T], T] {
	return Column[To, sql.Null[T], T]{ref: c.ref}
}

// RebindExistingNullable preserves schema nullability while changing scope.
func RebindExistingNullable[From, To, T any](c Column[From, sql.Null[T], T]) Column[To, sql.Null[T], T] {
	return Column[To, sql.Null[T], T]{ref: c.ref}
}

func (c Column[S, V, C]) Eq(value C) Predicate   { return comparison(c.ref, "=", value) }
func (c Column[S, V, C]) Ne(value C) Predicate   { return comparison(c.ref, "<>", value) }
func (c Column[S, V, C]) Gt(value C) Predicate   { return comparison(c.ref, ">", value) }
func (c Column[S, V, C]) Gte(value C) Predicate  { return comparison(c.ref, ">=", value) }
func (c Column[S, V, C]) Lt(value C) Predicate   { return comparison(c.ref, "<", value) }
func (c Column[S, V, C]) Lte(value C) Predicate  { return comparison(c.ref, "<=", value) }
func (c Column[S, V, C]) Like(value C) Predicate { return comparison(c.ref, "LIKE", value) }

func (c Column[S, V, C]) In(values ...C) Predicate {
	return listPredicate(c.ref, "IN", values)
}

func (c Column[S, V, C]) NotIn(values ...C) Predicate {
	return listPredicate(c.ref, "NOT IN", values)
}

func (c Column[S, V, C]) IsNull() Predicate {
	return Predicate{node: predicateNode{kind: nullComparison, left: c.ref, op: "IS NULL"}}
}

func (c Column[S, V, C]) IsNotNull() Predicate {
	return Predicate{node: predicateNode{kind: nullComparison, left: c.ref, op: "IS NOT NULL"}}
}

func (c Column[S, V, C]) Count() Aggregate[S, int64, int64] {
	return aggregate[S, int64, int64]("COUNT", c.ref)
}

func (c Column[S, V, C]) Min() Aggregate[S, sql.Null[C], C] {
	return aggregate[S, sql.Null[C], C]("MIN", c.ref)
}

func (c Column[S, V, C]) Max() Aggregate[S, sql.Null[C], C] {
	return aggregate[S, sql.Null[C], C]("MAX", c.ref)
}

// To assigns a value to this column in an UPDATE statement.
func (c Column[S, V, C]) To(value C) Assignment[S] {
	return Assignment[S]{column: c.ref, value: value}
}

// Value associates this column with a value in an INSERT statement.
func (c Column[S, V, C]) Value(value C) InsertValue[S] {
	return InsertValue[S]{column: c.ref, value: value}
}

func listPredicate[C any](column columnRef, operator string, values []C) Predicate {
	if len(values) == 0 {
		panic("tiqq: " + operator + " requires at least one value")
	}
	arguments := make([]any, len(values))
	for index, value := range values {
		arguments[index] = value
	}
	return Predicate{node: predicateNode{
		kind: listComparison, left: column, op: operator, values: arguments,
	}}
}

func (c Column[S, V, C]) selection() columnRef       { return c.ref }
func (c Column[S, V, C]) resultRef() columnRef       { return c.ref }
func (c Column[S, V, C]) resultValue(V)              {}
func (c Column[S, V, C]) comparisonRef(C) columnRef  { return c.ref }
func (c Column[S, V, C]) conflictColumn(S) columnRef { return c.ref }

// Selection is the closed set of selectable expressions for scope S.
type Selection interface {
	selection() columnRef
}
