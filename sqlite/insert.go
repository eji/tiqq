// Package sqlite provides SQLite-specific query extensions.
package sqlite

import "github.com/eji/tiqq"

// InsertQuery is the SQLite INSERT builder. SQLite-specific extensions belong here.
type InsertQuery[S any] struct {
	query tiqq.InsertQuery[S, tiqq.SQLiteMarker]
}

// NewInsert wraps a common SQLite INSERT. Intended for generated code.
func NewInsert[S any](query tiqq.InsertQuery[S, tiqq.SQLiteMarker]) InsertQuery[S] {
	return InsertQuery[S]{query: query}
}

func (query InsertQuery[S]) Values(values ...tiqq.InsertValue[S]) InsertQuery[S] {
	query.query = query.query.Values(values...)
	return query
}

func (query InsertQuery[S]) Build() (tiqq.Statement, error) { return query.query.Build() }
func (query InsertQuery[S]) MustBuild() tiqq.Statement      { return query.query.MustBuild() }
