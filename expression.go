package tiqq

import (
	"fmt"
	"strings"
)

type predicateNode struct {
	column columnRef
	op     string
	value  any
}

// Predicate carries the scope in which it is valid.
type Predicate[S any] struct{ node predicateNode }

func comparison[S any](column columnRef, op string, value any) Predicate[S] {
	return Predicate[S]{node: predicateNode{column: column, op: op, value: value}}
}

type joinNode struct {
	left  columnRef
	op    string
	right columnRef
}

// JoinCondition is created by On. Its type arguments prevent comparing
// columns with different comparison types.
type JoinCondition struct{ node joinNode }

func On[LS, LV, RS, RV, T any](left Column[LS, LV, T], right Column[RS, RV, T]) JoinCondition {
	return JoinCondition{node: joinNode{left: left.ref, op: "=", right: right.ref}}
}

func quoteIdent(s string) string { return `"` + strings.ReplaceAll(s, `"`, `""`) + `"` }

func renderColumn(c columnRef) string {
	return quoteIdent(c.qualifier) + "." + quoteIdent(c.name)
}

func renderJoin(j JoinCondition) string {
	return renderColumn(j.node.left) + " " + j.node.op + " " + renderColumn(j.node.right)
}

func renderPredicate[S any](p Predicate[S], arg int) (string, any) {
	return fmt.Sprintf("%s %s $%d", renderColumn(p.node.column), p.node.op, arg), p.node.value
}
