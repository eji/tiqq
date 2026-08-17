package tiqq

import (
	"fmt"
	"strings"
)

type expressionKind uint8

const (
	valueComparison expressionKind = iota
	columnComparison
)

type predicateNode struct {
	kind        expressionKind
	left        columnRef
	op          string
	rightColumn columnRef
	value       any
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

func quoteIdent(s string) string { return `"` + strings.ReplaceAll(s, `"`, `""`) + `"` }

func renderColumn(c columnRef) string {
	return quoteIdent(c.qualifier) + "." + quoteIdent(c.name)
}

func renderPredicate(p Predicate, nextArg *int) (string, []any) {
	if p.node.kind == columnComparison {
		return renderColumn(p.node.left) + " " + p.node.op + " " + renderColumn(p.node.rightColumn), nil
	}
	placeholder := *nextArg
	*nextArg++
	return fmt.Sprintf("%s %s $%d", renderColumn(p.node.left), p.node.op, placeholder), []any{p.node.value}
}

func predicateColumns(predicate Predicate) []columnRef {
	if predicate.node.kind == columnComparison {
		return []columnRef{predicate.node.left, predicate.node.rightColumn}
	}
	return []columnRef{predicate.node.left}
}
