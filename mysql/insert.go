// Package mysql provides MySQL-specific query extensions.
package mysql

import "github.com/eji/tiqq"

// InsertQuery is the MySQL INSERT builder. MySQL-specific extensions belong here.
type InsertQuery[S any] struct {
	query tiqq.InsertQuery[S, tiqq.MySQLMarker]
}

// NewInsert wraps a common MySQL INSERT. Intended for generated code.
func NewInsert[S any](query tiqq.InsertQuery[S, tiqq.MySQLMarker]) InsertQuery[S] {
	return InsertQuery[S]{query: query}
}

func (query InsertQuery[S]) Values(values ...tiqq.InsertValue[S]) InsertQuery[S] {
	query.query = query.query.Values(values...)
	return query
}

func (query InsertQuery[S]) Build() (tiqq.Statement, error) { return query.query.Build() }
func (query InsertQuery[S]) MustBuild() tiqq.Statement      { return query.query.MustBuild() }
