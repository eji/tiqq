package tiqq

import (
	"fmt"
	"strings"
)

// UpdateQuery builds a type-safe UPDATE statement for table scope S.
type UpdateQuery[S any] struct {
	table       tableSource
	assignments []Assignment[S]
	predicates  []Predicate
	allRows     bool
}

// NewUpdate constructs an UPDATE for a generated table.
func NewUpdate[S, C, NC any](table TableLike[C, NC]) UpdateQuery[S] {
	return UpdateQuery[S]{table: tableSourceOf(table.TiqqTableInfo())}
}

// Set appends assignments, allowing dynamic update construction.
func (query UpdateQuery[S]) Set(assignments ...Assignment[S]) UpdateQuery[S] {
	query.assignments = appendCopy(query.assignments, assignments...)
	return query
}

func (query UpdateQuery[S]) Where(predicates ...Predicate) UpdateQuery[S] {
	query.predicates = appendCopy(query.predicates, predicates...)
	return query
}

// AllRows explicitly permits an UPDATE without a WHERE clause.
func (query UpdateQuery[S]) AllRows() UpdateQuery[S] {
	query.allRows = true
	return query
}

func (query UpdateQuery[S]) Build() (Statement, error) {
	renderer, err := rendererFor(query.table.ref)
	if err != nil {
		return Statement{}, err
	}
	if len(query.assignments) == 0 {
		return Statement{}, fmt.Errorf("tiqq: UPDATE requires at least one SET assignment")
	}
	if len(query.predicates) == 0 && !query.allRows {
		return Statement{}, fmt.Errorf("tiqq: UPDATE requires WHERE or AllRows")
	}
	allowed := map[string]bool{query.table.ref.qualifier(): true}
	columns := make([]columnRef, len(query.assignments))
	for index, assignment := range query.assignments {
		if assignment.excluded || assignment.inserted {
			return Statement{}, fmt.Errorf("tiqq: UPDATE SET does not accept incoming-row assignments")
		}
		columns[index] = assignment.column
	}
	if err := validateColumns("SET", columns, allowed); err != nil {
		return Statement{}, err
	}
	for _, predicate := range query.predicates {
		if err := validateColumns("WHERE", predicateColumns(predicate), allowed); err != nil {
			return Statement{}, err
		}
	}

	var builder strings.Builder
	builder.WriteString("UPDATE ")
	renderTable(renderer, &builder, query.table.ref)
	builder.WriteString(" SET ")
	args := make([]any, 0, len(query.assignments)+len(query.predicates))
	nextArg := 1
	for index, assignment := range query.assignments {
		if index > 0 {
			builder.WriteString(", ")
		}
		builder.WriteString(renderer.quoteIdentifier(assignment.column.name))
		builder.WriteString(" = ")
		builder.WriteString(renderer.placeholder(nextArg))
		nextArg++
		args = append(args, assignment.value)
	}
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
	return newStatement(builder.String(), args, nil), nil
}

// MustBuild builds an UPDATE statement and panics if validation fails.
func (query UpdateQuery[S]) MustBuild() Statement {
	statement, err := query.Build()
	if err != nil {
		panic(err)
	}
	return statement
}
