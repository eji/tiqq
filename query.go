package tiqq

import (
	"strings"
)

type source struct {
	baseTable string
	baseAlias string
	joins     []joinClause
}

type joinClause struct {
	kind, table, alias string
	on                 JoinCondition
}

// NewAliasedLeftJoinSource constructs a self-join-capable source. Intended for
// generated alias table types.
func NewAliasedLeftJoinSource(leftTable, leftAlias, rightTable, rightAlias string, on JoinCondition) Source {
	return Source{source{
		baseTable: leftTable,
		baseAlias: leftAlias,
		joins: []joinClause{{
			kind: "LEFT JOIN", table: rightTable, alias: rightAlias, on: on,
		}},
	}}
}

// NewAliasedInnerJoinSource constructs an aliased INNER JOIN source.
func NewAliasedInnerJoinSource(leftTable, leftAlias, rightTable, rightAlias string, on JoinCondition) Source {
	return Source{source{
		baseTable: leftTable,
		baseAlias: leftAlias,
		joins: []joinClause{{
			kind: "INNER JOIN", table: rightTable, alias: rightAlias, on: on,
		}},
	}}
}

// NewLeftJoinSource is intended for generated join types.
func NewLeftJoinSource(left, right string, on JoinCondition) Source {
	return Source{source{baseTable: left, joins: []joinClause{{kind: "LEFT JOIN", table: right, on: on}}}}
}

// NewInnerJoinSource is intended for generated join types.
func NewInnerJoinSource(left, right string, on JoinCondition) Source {
	return Source{source{baseTable: left, joins: []joinClause{{kind: "INNER JOIN", table: right, on: on}}}}
}

// Source is an immutable FROM/JOIN tree used by generated query types.
type Source struct{ value source }

func NewQuery[S any](from Source) Query[S] { return Query[S]{from: from.value} }

// Query builds a SELECT while retaining typed projection metadata.
type Query[S any] struct {
	from        source
	predicates  []Predicate[S]
	projections []columnRef
}

func (q Query[S]) Where(predicates ...Predicate[S]) Query[S] {
	q.predicates = appendCopy(q.predicates, predicates...)
	return q
}

func (q Query[S]) Select(columns ...Selection[S]) Query[S] {
	q.projections = make([]columnRef, len(columns))
	for i, column := range columns {
		q.projections[i] = column.selection()
	}
	return q
}

func (q Query[S]) Build() Statement {
	if len(q.projections) == 0 {
		panic("tiqq: SELECT requires at least one column")
	}
	var b strings.Builder
	b.WriteString("SELECT ")
	for i, c := range q.projections {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(renderColumn(c))
	}
	b.WriteString(" FROM ")
	b.WriteString(quoteIdent(q.from.baseTable))
	if q.from.baseAlias != "" {
		b.WriteString(" AS ")
		b.WriteString(quoteIdent(q.from.baseAlias))
	}
	for _, join := range q.from.joins {
		b.WriteByte(' ')
		b.WriteString(join.kind)
		b.WriteByte(' ')
		b.WriteString(quoteIdent(join.table))
		if join.alias != "" {
			b.WriteString(" AS ")
			b.WriteString(quoteIdent(join.alias))
		}
		b.WriteString(" ON ")
		b.WriteString(renderJoin(join.on))
	}
	args := make([]any, 0, len(q.predicates))
	if len(q.predicates) > 0 {
		b.WriteString(" WHERE ")
		for i, predicate := range q.predicates {
			if i > 0 {
				b.WriteString(" AND ")
			}
			sql, arg := renderPredicate(predicate, i+1)
			b.WriteString(sql)
			args = append(args, arg)
		}
	}
	return newStatement(b.String(), args, q.projections)
}

func appendCopy[T any](base []T, values ...T) []T {
	out := make([]T, len(base), len(base)+len(values))
	copy(out, base)
	return append(out, values...)
}
