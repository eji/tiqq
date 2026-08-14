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
}

// RequiredColumn constructs a non-nullable column. It is primarily intended
// for generated code.
func RequiredColumn[S, T any](table, name string) Column[S, T, T] {
	return Column[S, T, T]{ref: columnRef{id: table + "." + name, qualifier: table, name: name}}
}

// RebindRequired changes a column's predicate scope. Intended for generated joins.
func RebindRequired[From, To, T any](c Column[From, T, T]) Column[To, T, T] {
	return Column[To, T, T]{ref: c.ref}
}

// RebindNullable changes scope and makes an outer-joined column nullable.
func RebindNullable[From, To, T any](c Column[From, T, T]) Column[To, sql.Null[T], T] {
	return Column[To, sql.Null[T], T]{ref: c.ref}
}

func (c Column[S, V, C]) Eq(value C) Predicate[S]   { return comparison[S](c.ref, "=", value) }
func (c Column[S, V, C]) Ne(value C) Predicate[S]   { return comparison[S](c.ref, "<>", value) }
func (c Column[S, V, C]) Gt(value C) Predicate[S]   { return comparison[S](c.ref, ">", value) }
func (c Column[S, V, C]) Gte(value C) Predicate[S]  { return comparison[S](c.ref, ">=", value) }
func (c Column[S, V, C]) Lt(value C) Predicate[S]   { return comparison[S](c.ref, "<", value) }
func (c Column[S, V, C]) Lte(value C) Predicate[S]  { return comparison[S](c.ref, "<=", value) }
func (c Column[S, V, C]) Like(value C) Predicate[S] { return comparison[S](c.ref, "LIKE", value) }

func (c Column[S, V, C]) selection() columnRef { return c.ref }

// Selection is the closed set of selectable expressions for scope S.
type Selection[S any] interface {
	selection() columnRef
}
