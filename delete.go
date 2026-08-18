package tiqq

import (
	"fmt"
	"strings"
)

// DeleteQuery builds a type-safe DELETE statement for table scope S.
type DeleteQuery[S any] struct {
	table      tableSource
	predicates []Predicate
	allRows    bool
	returning  []columnRef
	returns    bool
}

// NewDelete constructs a DELETE for a generated table.
func NewDelete[S, C, NC any](table TableLike[C, NC]) DeleteQuery[S] {
	return DeleteQuery[S]{table: tableSourceOf(table.TiqqTableInfo())}
}

func (query DeleteQuery[S]) Where(predicates ...Predicate) DeleteQuery[S] {
	query.predicates = appendCopy(query.predicates, predicates...)
	return query
}

// AllRows explicitly permits a DELETE without a WHERE clause.
func (query DeleteQuery[S]) AllRows() DeleteQuery[S] {
	query.allRows = true
	return query
}

// Returning selects values produced by the deleted rows.
func (query DeleteQuery[S]) Returning(columns ...Selection) DeleteQuery[S] {
	query.returns = true
	query.returning = selections(columns)
	return query
}

func (query DeleteQuery[S]) Build() (Statement, error) {
	renderer, err := rendererFor(query.table.ref)
	if err != nil {
		return Statement{}, err
	}
	if len(query.predicates) == 0 && !query.allRows {
		return Statement{}, fmt.Errorf("tiqq: DELETE requires WHERE or AllRows")
	}
	if err := validateReturning(renderer, query.table.ref, query.returning, query.returns); err != nil {
		return Statement{}, err
	}
	allowed := map[string]bool{query.table.ref.qualifier(): true}
	for _, predicate := range query.predicates {
		columns := predicateColumns(predicate)
		if err := validateColumns("WHERE", columns, allowed); err != nil {
			return Statement{}, err
		}
		if err := validateNoAggregates("WHERE", columns); err != nil {
			return Statement{}, err
		}
	}

	var builder strings.Builder
	builder.WriteString("DELETE FROM ")
	renderTable(renderer, &builder, query.table.ref)
	args := make([]any, 0, len(query.predicates))
	nextArg := 1
	if len(query.predicates) > 0 {
		builder.WriteString(" WHERE ")
		for index, predicate := range query.predicates {
			if index > 0 {
				builder.WriteString(" AND ")
			}
			text, values := renderPredicate(renderer, predicate, &nextArg)
			builder.WriteString(text)
			args = append(args, values...)
		}
	}
	renderReturning(renderer, &builder, query.returning)
	return newStatement(builder.String(), args, query.returning), nil
}

// MustBuild builds a DELETE statement and panics if validation fails.
func (query DeleteQuery[S]) MustBuild() Statement {
	statement, err := query.Build()
	if err != nil {
		panic(err)
	}
	return statement
}
