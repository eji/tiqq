package tiqq

import (
	"fmt"
	"strings"
)

type expressionKind uint8

const (
	valueComparison expressionKind = iota
	columnComparison
	listComparison
	nullComparison
	logicalExpression
	negatedExpression
)

type predicateNode struct {
	kind        expressionKind
	left        columnRef
	op          string
	rightColumn columnRef
	value       any
	values      []any
	children    []Predicate
}

// Predicate is shared by ON and WHERE. Query membership is validated by Build.
type Predicate struct{ node predicateNode }

func comparison(column columnRef, op string, value any) Predicate {
	return Predicate{node: predicateNode{kind: valueComparison, left: column, op: op, value: value}}
}

// Eq compares two columns with the same comparison type.
func Eq[LS, LV, RS, RV, T any](left Column[LS, LV, T], right Column[RS, RV, T]) Predicate {
	return Predicate{node: predicateNode{
		kind: columnComparison, left: left.ref, op: "=", rightColumn: right.ref,
	}}
}

// And combines predicates with SQL AND.
func And(predicates ...Predicate) Predicate {
	return logical("AND", predicates)
}

// Or combines predicates with SQL OR.
func Or(predicates ...Predicate) Predicate {
	return logical("OR", predicates)
}

// Not negates a predicate.
func Not(predicate Predicate) Predicate {
	return Predicate{node: predicateNode{kind: negatedExpression, children: []Predicate{predicate}}}
}

func logical(operator string, predicates []Predicate) Predicate {
	if len(predicates) == 0 {
		panic("tiqq: " + operator + " requires at least one predicate")
	}
	return Predicate{node: predicateNode{
		kind: logicalExpression, op: operator, children: append([]Predicate(nil), predicates...),
	}}
}

func quoteIdent(s string) string { return `"` + strings.ReplaceAll(s, `"`, `""`) + `"` }

func renderColumn(c columnRef) string {
	return quoteIdent(c.qualifier) + "." + quoteIdent(c.name)
}

func renderPredicate(p Predicate, nextArg *int) (string, []any) {
	switch p.node.kind {
	case columnComparison:
		return renderColumn(p.node.left) + " " + p.node.op + " " + renderColumn(p.node.rightColumn), nil
	case listComparison:
		placeholders := make([]string, len(p.node.values))
		for index := range p.node.values {
			placeholders[index] = fmt.Sprintf("$%d", *nextArg)
			*nextArg = *nextArg + 1
		}
		return renderColumn(p.node.left) + " " + p.node.op + " (" + strings.Join(placeholders, ", ") + ")", append([]any(nil), p.node.values...)
	case nullComparison:
		return renderColumn(p.node.left) + " " + p.node.op, nil
	case logicalExpression:
		parts := make([]string, len(p.node.children))
		var arguments []any
		for index, child := range p.node.children {
			text, values := renderPredicate(child, nextArg)
			parts[index] = text
			arguments = append(arguments, values...)
		}
		return "(" + strings.Join(parts, " "+p.node.op+" ") + ")", arguments
	case negatedExpression:
		text, arguments := renderPredicate(p.node.children[0], nextArg)
		return "NOT (" + text + ")", arguments
	case valueComparison:
		placeholder := *nextArg
		*nextArg = *nextArg + 1
		return fmt.Sprintf("%s %s $%d", renderColumn(p.node.left), p.node.op, placeholder), []any{p.node.value}
	default:
		panic("tiqq: unknown predicate expression")
	}
}

func predicateColumns(predicate Predicate) []columnRef {
	switch predicate.node.kind {
	case columnComparison:
		return []columnRef{predicate.node.left, predicate.node.rightColumn}
	case logicalExpression, negatedExpression:
		var columns []columnRef
		for _, child := range predicate.node.children {
			columns = append(columns, predicateColumns(child)...)
		}
		return columns
	default:
		return []columnRef{predicate.node.left}
	}
}
