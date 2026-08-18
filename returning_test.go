package tiqq_test

import (
	"database/sql"
	"encoding/json/jsontext"
	"testing"
	"uuid"

	"github.com/eji/tiqq"
	. "github.com/eji/tiqq/example/schema"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

func TestReturningBuild(t *testing.T) {
	tests := map[string]struct {
		build    func() tiqq.Statement
		wantSQL  string
		wantArgs []any
	}{
		"insert": {
			build: func() tiqq.Statement {
				return UserTable.Insert().Values(
					UserTable.ID.Value(int64(7)),
					UserTable.Name.Value("Alice"),
				).Returning(UserTable.ID, UserTable.Name).MustBuild()
			},
			wantSQL:  `INSERT INTO "users" ("id", "name") VALUES ($1, $2) RETURNING "id", "name"`,
			wantArgs: []any{int64(7), "Alice"},
		},
		"update": {
			build: func() tiqq.Statement {
				return UserTable.Update().Set(UserTable.Name.To("Alice")).
					Where(UserTable.ID.Eq(int64(7))).
					Returning(UserTable.ID, UserTable.Name).
					MustBuild()
			},
			wantSQL:  `UPDATE "users" SET "name" = $1 WHERE "users"."id" = $2 RETURNING "id", "name"`,
			wantArgs: []any{"Alice", int64(7)},
		},
		"delete": {
			build: func() tiqq.Statement {
				return UserTable.Delete().Where(UserTable.ID.Eq(int64(7))).
					Returning(UserTable.ID, UserTable.Name).
					MustBuild()
			},
			wantSQL:  `DELETE FROM "users" WHERE "users"."id" = $1 RETURNING "id", "name"`,
			wantArgs: []any{int64(7)},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			statement := test.build()
			require.Equal(t, test.wantSQL, statement.SQL())
			require.Equal(t, test.wantArgs, statement.Args())
		})
	}
}

func TestReturningRetainsTypedProjection(t *testing.T) {
	statement := UserTable.Insert().Values(
		UserTable.ID.Value(int64(7)),
		UserTable.Name.Value("Alice"),
	).Returning(UserTable.ID, UserTable.Name).MustBuild()
	row, err := tiqq.NewRow(statement, int64(7), "Alice")
	id, idErr := row.Get(UserTable.ID)
	name, nameErr := row.Get(UserTable.Name)

	require.NoError(t, err)
	require.NoError(t, idErr)
	require.NoError(t, nameErr)
	require.Equal(t, int64(7), id)
	require.Equal(t, "Alice", name)
}

func TestReturningBuildValidation(t *testing.T) {
	tests := map[string]struct {
		build func() error
		want  string
	}{
		"column is required": {
			build: func() error {
				_, err := UserTable.Delete().AllRows().Returning().Build()
				return err
			},
			want: "tiqq: RETURNING requires at least one column",
		},
		"column must belong to table": {
			build: func() error {
				_, err := UserTable.Delete().AllRows().Returning(AddressTable.ID).Build()
				return err
			},
			want: "tiqq: RETURNING column addresses.id is not in query scope",
		},
		"aggregate is rejected": {
			build: func() error {
				_, err := UserTable.Delete().AllRows().Returning(UserTable.ID.Count()).Build()
				return err
			},
			want: "tiqq: RETURNING does not accept aggregate COUNT(users.id)",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			require.EqualError(t, test.build(), test.want)
		})
	}
}

func TestSQLiteReturningRoundTrip(t *testing.T) {
	database, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, database.Close()) })
	_, err = database.Exec(`CREATE TABLE events (id UUID PRIMARY KEY, payload JSON NOT NULL)`)
	require.NoError(t, err)

	table := newStandardTypeTable(tiqq.SQLite)
	id := uuid.MustParse("01890a5d-ac96-774b-bcce-b302099a8057")
	payload := jsontext.Value(`{"name":"Alice"}`)
	statement := tiqq.NewInsert[standardTypeScope, tiqq.SQLiteMarker](
		table.ref, []string{"id", "payload"}, []string{"id", "payload"}, [][]string{{"id"}},
	).Values(
		table.columns.ID.Value(id), table.columns.Payload.Value(payload),
	).Returning(table.columns.ID, table.columns.Payload).MustBuild()
	row, err := tiqq.ScanRow(
		database.QueryRow(statement.SQL(), statement.Args()...), statement,
	)
	gotID, idErr := row.Get(table.columns.ID)
	gotPayload, payloadErr := row.Get(table.columns.Payload)

	require.NoError(t, err)
	require.NoError(t, idErr)
	require.NoError(t, payloadErr)
	require.Equal(t, id, gotID)
	require.Equal(t, payload, gotPayload)
}
