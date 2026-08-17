package tiqq

import (
	"fmt"
	"strings"
)

// InsertQuery builds a type-safe single-row or bulk INSERT for scope S and dialect D.
type InsertQuery[S any, D InsertDialect] struct {
	table      TableRef
	rows       [][]InsertValue[S]
	insertable map[string]bool
	required   []string
	uniqueKeys [][]string
	conflict   *insertConflict[S]
	duplicate  []Assignment[S]
}

type insertConflict[S any] struct {
	target      []columnRef
	doNothing   bool
	assignments []Assignment[S]
}

// NewInsert constructs an INSERT for generated code.
func NewInsert[S any, D InsertDialect](table TableRef, insertable, required []string, uniqueKeys [][]string) InsertQuery[S, D] {
	allowed := make(map[string]bool, len(insertable))
	for _, column := range insertable {
		allowed[column] = true
	}
	keys := make([][]string, len(uniqueKeys))
	for index, key := range uniqueKeys {
		keys[index] = append([]string(nil), key...)
	}
	return InsertQuery[S, D]{
		table: table, insertable: allowed,
		required: append([]string(nil), required...), uniqueKeys: keys,
	}
}

// Values appends one row to a multi-row INSERT.
func (query InsertQuery[S, D]) Values(values ...InsertValue[S]) InsertQuery[S, D] {
	query.rows = appendCopy(query.rows, appendCopy([]InsertValue[S](nil), values...))
	return query
}

func (query InsertQuery[S, D]) Build() (Statement, error) {
	renderer, err := rendererFor(query.table)
	if err != nil {
		return Statement{}, err
	}
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
	if query.conflict != nil && len(query.duplicate) > 0 {
		return Statement{}, fmt.Errorf("tiqq: INSERT cannot combine ON CONFLICT and ON DUPLICATE KEY UPDATE")
	}
	if err := query.validateConflict(renderer); err != nil {
		return Statement{}, err
	}
	if err := query.validateDuplicateKey(); err != nil {
		return Statement{}, err
	}

	var builder strings.Builder
	builder.WriteString("INSERT INTO ")
	renderTable(renderer, &builder, query.table)
	builder.WriteString(" (")
	for index, value := range query.rows[0] {
		if index > 0 {
			builder.WriteString(", ")
		}
		builder.WriteString(renderer.quoteIdentifier(value.column.name))
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
			placeholders[index] = renderer.placeholder(nextArg)
			nextArg++
			args = append(args, value.value)
		}
		builder.WriteByte('(')
		builder.WriteString(strings.Join(placeholders, ", "))
		builder.WriteByte(')')
	}
	if len(query.duplicate) > 0 {
		builder.WriteString(" AS ")
		builder.WriteString(renderer.quoteIdentifier("tiqq_new"))
	}
	query.renderConflict(renderer, &builder, &args, &nextArg)
	query.renderDuplicateKey(renderer, &builder, &args, &nextArg)
	return newStatement(builder.String(), args, nil), nil
}

func (query InsertQuery[S, D]) renderDuplicateKey(renderer sqlRenderer, builder *strings.Builder, args *[]any, nextArg *int) {
	if len(query.duplicate) == 0 {
		return
	}
	builder.WriteString(" ON DUPLICATE KEY UPDATE ")
	for index, assignment := range query.duplicate {
		if index > 0 {
			builder.WriteString(", ")
		}
		builder.WriteString(renderer.quoteIdentifier(assignment.column.name))
		builder.WriteString(" = ")
		if assignment.inserted {
			builder.WriteString(renderer.quoteIdentifier("tiqq_new"))
			builder.WriteByte('.')
			builder.WriteString(renderer.quoteIdentifier(assignment.column.name))
		} else {
			builder.WriteString(renderer.placeholder(*nextArg))
			*nextArg++
			*args = append(*args, assignment.value)
		}
	}
}

func (query InsertQuery[S, D]) renderConflict(renderer sqlRenderer, builder *strings.Builder, args *[]any, nextArg *int) {
	if query.conflict == nil {
		return
	}
	builder.WriteString(" ON CONFLICT")
	if len(query.conflict.target) > 0 {
		builder.WriteString(" (")
		for index, column := range query.conflict.target {
			if index > 0 {
				builder.WriteString(", ")
			}
			builder.WriteString(renderer.quoteIdentifier(column.name))
		}
		builder.WriteByte(')')
	}
	if query.conflict.doNothing {
		builder.WriteString(" DO NOTHING")
		return
	}
	builder.WriteString(" DO UPDATE SET ")
	for index, assignment := range query.conflict.assignments {
		if index > 0 {
			builder.WriteString(", ")
		}
		builder.WriteString(renderer.quoteIdentifier(assignment.column.name))
		builder.WriteString(" = ")
		if assignment.excluded {
			builder.WriteString("EXCLUDED.")
			builder.WriteString(renderer.quoteIdentifier(assignment.column.name))
		} else {
			builder.WriteString(renderer.placeholder(*nextArg))
			*nextArg++
			*args = append(*args, assignment.value)
		}
	}
}

func (query InsertQuery[S, D]) validateConflict(renderer sqlRenderer) error {
	if query.conflict == nil {
		return nil
	}
	if len(query.conflict.target) == 0 {
		if query.conflict.doNothing {
			return nil
		}
		if renderer.name() != "sqlite" {
			return fmt.Errorf("tiqq: ON CONFLICT DO UPDATE requires a conflict target")
		}
	}
	seen := make(map[string]bool, len(query.conflict.target))
	for _, column := range query.conflict.target {
		if column.qualifier != query.table.qualifier() {
			return fmt.Errorf("tiqq: ON CONFLICT column %s is not in query scope", column.id)
		}
		if seen[column.name] {
			return fmt.Errorf("tiqq: ON CONFLICT column %s is specified more than once", column.id)
		}
		seen[column.name] = true
	}
	if len(seen) > 0 {
		matched := false
		for _, key := range query.uniqueKeys {
			keyColumns := make(map[string]bool, len(key))
			for _, column := range key {
				keyColumns[column] = true
			}
			matched = matched || len(keyColumns) == len(seen) && sameColumnSet(keyColumns, seen)
		}
		if !matched {
			return fmt.Errorf("tiqq: ON CONFLICT target does not match a primary key or unique constraint")
		}
	}
	if !query.conflict.doNothing && len(query.conflict.assignments) == 0 {
		return fmt.Errorf("tiqq: ON CONFLICT DO UPDATE requires at least one assignment")
	}
	assigned := make(map[string]bool, len(query.conflict.assignments))
	for _, assignment := range query.conflict.assignments {
		if assignment.column.qualifier != query.table.qualifier() {
			return fmt.Errorf("tiqq: ON CONFLICT assignment column %s is not in query scope", assignment.column.id)
		}
		if !query.insertable[assignment.column.name] {
			return fmt.Errorf("tiqq: column %s is not insertable", assignment.column.id)
		}
		if assigned[assignment.column.name] {
			return fmt.Errorf("tiqq: ON CONFLICT assignment column %s is specified more than once", assignment.column.id)
		}
		assigned[assignment.column.name] = true
	}
	return nil
}

func (query InsertQuery[S, D]) validateDuplicateKey() error {
	if query.duplicate == nil {
		return nil
	}
	if len(query.duplicate) == 0 {
		return fmt.Errorf("tiqq: ON DUPLICATE KEY UPDATE requires at least one assignment")
	}
	assigned := make(map[string]bool, len(query.duplicate))
	for _, assignment := range query.duplicate {
		if assignment.column.qualifier != query.table.qualifier() {
			return fmt.Errorf("tiqq: ON DUPLICATE KEY assignment column %s is not in query scope", assignment.column.id)
		}
		if !query.insertable[assignment.column.name] {
			return fmt.Errorf("tiqq: column %s is not insertable", assignment.column.id)
		}
		if assignment.excluded {
			return fmt.Errorf("tiqq: ON DUPLICATE KEY UPDATE does not accept EXCLUDED assignments")
		}
		if assigned[assignment.column.name] {
			return fmt.Errorf("tiqq: ON DUPLICATE KEY assignment column %s is specified more than once", assignment.column.id)
		}
		assigned[assignment.column.name] = true
	}
	return nil
}

func sameColumnSet(left, right map[string]bool) bool {
	for column := range left {
		if !right[column] {
			return false
		}
	}
	return true
}

// WithConflictDoNothing is intended for dialect packages.
func WithConflictDoNothing[S any, D InsertDialect](query InsertQuery[S, D], columns ...ConflictColumn[S]) InsertQuery[S, D] {
	query.conflict = &insertConflict[S]{target: conflictColumns(columns), doNothing: true}
	return query
}

// WithConflictDoUpdate is intended for dialect packages.
func WithConflictDoUpdate[S any, D InsertDialect](query InsertQuery[S, D], columns []ConflictColumn[S], values ...UpdateValue[S]) InsertQuery[S, D] {
	assignments := make([]Assignment[S], len(values))
	for index, value := range values {
		assignments[index] = value.updateValue(*new(S))
	}
	query.conflict = &insertConflict[S]{target: conflictColumns(columns), assignments: assignments}
	return query
}

func conflictColumns[S any](columns []ConflictColumn[S]) []columnRef {
	result := make([]columnRef, len(columns))
	for index, column := range columns {
		result[index] = column.conflictColumn(*new(S))
	}
	return result
}

// ExcludedAssignment creates an SQL EXCLUDED incoming-row assignment for dialect packages.
func ExcludedAssignment[S any](column ConflictColumn[S]) Assignment[S] {
	return Assignment[S]{column: column.conflictColumn(*new(S)), excluded: true}
}

// WithDuplicateKeyDoUpdate is intended for the MySQL dialect package.
func WithDuplicateKeyDoUpdate[S any](query InsertQuery[S, MySQLMarker], values ...UpdateValue[S]) InsertQuery[S, MySQLMarker] {
	query.duplicate = make([]Assignment[S], len(values))
	for index, value := range values {
		query.duplicate[index] = value.updateValue(*new(S))
	}
	return query
}

// InsertedAssignment creates a MySQL incoming-row assignment for the dialect package.
func InsertedAssignment[S any](column ConflictColumn[S]) Assignment[S] {
	return Assignment[S]{column: column.conflictColumn(*new(S)), inserted: true}
}

// MustBuild builds an INSERT statement and panics if validation fails.
func (query InsertQuery[S, D]) MustBuild() Statement {
	statement, err := query.Build()
	if err != nil {
		panic(err)
	}
	return statement
}
