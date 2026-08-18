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

func (query InsertQuery[S]) Returning(columns ...tiqq.Selection) InsertQuery[S] {
	query.query = query.query.Returning(columns...)
	return query
}

func (query InsertQuery[S]) Build() (tiqq.Statement, error) { return query.query.Build() }
func (query InsertQuery[S]) MustBuild() tiqq.Statement      { return query.query.MustBuild() }

func (query InsertQuery[S]) OnDuplicateKey() DuplicateKeyBuilder[S] {
	return DuplicateKeyBuilder[S]{query: query}
}

// DuplicateKeyBuilder requires an explicit MySQL duplicate-key action.
type DuplicateKeyBuilder[S any] struct {
	query InsertQuery[S]
}

func (builder DuplicateKeyBuilder[S]) DoUpdate(values ...tiqq.UpdateValue[S]) InsertQuery[S] {
	builder.query.query = tiqq.WithDuplicateKeyDoUpdate(builder.query.query, values...)
	return builder.query
}

// Inserted updates a column from the row proposed for insertion.
func Inserted[S any](column tiqq.ConflictColumn[S]) tiqq.UpdateValue[S] {
	return tiqq.InsertedAssignment(column)
}
