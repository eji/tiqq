package sqlite_test

import (
	"testing"

	"github.com/eji/tiqq"
	"github.com/eji/tiqq/sqlite"
	"github.com/stretchr/testify/require"
)

type userScope struct{}

func TestInsertBuild(t *testing.T) {
	table := tiqq.NewSchemaInfo(tiqq.SQLite).Table("users")
	name := tiqq.RequiredColumn[userScope, string]("users", "name")
	query := sqlite.NewInsert(tiqq.NewInsert[userScope, tiqq.SQLiteMarker](
		table, []string{"name"}, []string{"name"}, nil,
	))

	statement := query.Values(name.Value("Alice")).MustBuild()

	require.Equal(t, `INSERT INTO "users" ("name") VALUES (?)`, statement.SQL())
	require.Equal(t, []any{"Alice"}, statement.Args())
}
