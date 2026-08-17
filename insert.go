package tiqq

import (
	"fmt"
	"strings"
)

// InsertQuery builds a type-safe single-row or bulk INSERT for scope S.
type InsertQuery[S any] struct {
	table      TableRef
	rows       [][]InsertValue[S]
	insertable map[string]bool
	required   []string
}

// NewInsert constructs an INSERT for generated code.
func NewInsert[S any](table TableRef, insertable, required []string, values ...InsertValue[S]) InsertQuery[S] {
	allowed := make(map[string]bool, len(insertable))
	for _, column := range insertable {
		allowed[column] = true
	}
	return InsertQuery[S]{
		table: table, rows: [][]InsertValue[S]{appendCopy([]InsertValue[S](nil), values...)},
		insertable: allowed, required: append([]string(nil), required...),
	}
}

// Values appends one row to a multi-row INSERT.
func (query InsertQuery[S]) Values(values ...InsertValue[S]) InsertQuery[S] {
	query.rows = appendCopy(query.rows, appendCopy([]InsertValue[S](nil), values...))
	return query
}

func (query InsertQuery[S]) Build() (Statement, error) {
	if len(query.rows) == 0 || len(query.rows[0]) == 0 {
		return Statement{}, fmt.Errorf("tiqq: INSERT requires at least one value")
	}
	firstColumns := make([]string, len(query.rows[0]))
	for rowIndex, row := range query.rows {
		if len(row) == 0 {
			return Statement{}, fmt.Errorf("tiqq: INSERT row %d requires at least one value", rowIndex+1)
		}
		if len(row) != len(query.rows[0]) {
			return Statement{}, fmt.Errorf("tiqq: INSERT row %d columns do not match the first row", rowIndex+1)
		}
		seen := make(map[string]bool, len(row))
		for columnIndex, value := range row {
			if value.column.qualifier != query.table.qualifier() {
				return Statement{}, fmt.Errorf("tiqq: INSERT column %s is not in query scope", value.column.id)
			}
			if !query.insertable[value.column.name] {
				return Statement{}, fmt.Errorf("tiqq: column %s is not insertable", value.column.id)
			}
			if seen[value.column.name] {
				return Statement{}, fmt.Errorf("tiqq: INSERT column %s is specified more than once", value.column.id)
			}
			if rowIndex == 0 {
				firstColumns[columnIndex] = value.column.name
			} else if value.column.name != firstColumns[columnIndex] {
				return Statement{}, fmt.Errorf("tiqq: INSERT row %d columns do not match the first row", rowIndex+1)
			}
			seen[value.column.name] = true
		}
		for _, required := range query.required {
			if !seen[required] {
				return Statement{}, fmt.Errorf("tiqq: INSERT requires column %s", required)
			}
		}
	}

	var builder strings.Builder
	builder.WriteString("INSERT INTO ")
	renderTable(&builder, query.table)
	builder.WriteString(" (")
	for index, value := range query.rows[0] {
		if index > 0 {
			builder.WriteString(", ")
		}
		builder.WriteString(quoteIdent(value.column.name))
	}
	builder.WriteString(") VALUES ")
	args := make([]any, 0, len(query.rows)*len(query.rows[0]))
	nextArg := 1
	for rowIndex, row := range query.rows {
		if rowIndex > 0 {
			builder.WriteString(", ")
		}
		placeholders := make([]string, len(row))
		for index, value := range row {
			placeholders[index] = fmt.Sprintf("$%d", nextArg)
			nextArg++
			args = append(args, value.value)
		}
		builder.WriteByte('(')
		builder.WriteString(strings.Join(placeholders, ", "))
		builder.WriteByte(')')
	}
	return newStatement(builder.String(), args, nil), nil
}

// MustBuild builds an INSERT statement and panics if validation fails.
func (query InsertQuery[S]) MustBuild() Statement {
	statement, err := query.Build()
	if err != nil {
		panic(err)
	}
	return statement
}
