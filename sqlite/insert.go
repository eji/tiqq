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

func (query InsertQuery[S]) Returning(columns ...tiqq.Selection) InsertQuery[S] {
	query.query = query.query.Returning(columns...)
	return query
}

func (query InsertQuery[S]) Build() (tiqq.Statement, error) { return query.query.Build() }
func (query InsertQuery[S]) MustBuild() tiqq.Statement      { return query.query.MustBuild() }

func (query InsertQuery[S]) OnConflict(columns ...tiqq.ConflictColumn[S]) ConflictBuilder[S] {
	return ConflictBuilder[S]{query: query, columns: append([]tiqq.ConflictColumn[S](nil), columns...)}
}

// ConflictBuilder requires an explicit SQLite conflict action.
type ConflictBuilder[S any] struct {
	query   InsertQuery[S]
	columns []tiqq.ConflictColumn[S]
}

func (builder ConflictBuilder[S]) DoNothing() InsertQuery[S] {
	builder.query.query = tiqq.WithConflictDoNothing(builder.query.query, builder.columns...)
	return builder.query
}

func (builder ConflictBuilder[S]) DoUpdate(values ...tiqq.UpdateValue[S]) InsertQuery[S] {
	builder.query.query = tiqq.WithConflictDoUpdate(builder.query.query, builder.columns, values...)
	return builder.query
}

// Excluded updates a column from the row proposed for insertion.
func Excluded[S any](column tiqq.ConflictColumn[S]) tiqq.UpdateValue[S] {
	return tiqq.ExcludedAssignment(column)
}
