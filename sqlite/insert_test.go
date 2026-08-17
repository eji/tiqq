package sqlite_test

import (
	"database/sql"
	"testing"

	"github.com/eji/tiqq"
	querysqlite "github.com/eji/tiqq/sqlite"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

type userScope struct{}

func TestOnConflictBuild(t *testing.T) {
	table := tiqq.NewSchemaInfo(tiqq.SQLite).Table("users")
	id := tiqq.RequiredNumericColumn[userScope, int64, int64, float64]("users", "id")
	email := tiqq.RequiredColumn[userScope, string]("users", "email")
	name := tiqq.RequiredColumn[userScope, string]("users", "name")
	query := querysqlite.NewInsert(tiqq.NewInsert[userScope, tiqq.SQLiteMarker](
		table, []string{"id", "email", "name"}, []string{"id", "email", "name"},
		[][]string{{"id"}, {"email"}},
	))
	tests := map[string]struct {
		query    querysqlite.InsertQuery[userScope]
		wantSQL  string
		wantArgs []any
	}{
		"insert": {
			query:    query.Values(id.Value(int64(1)), email.Value("alice@example.com"), name.Value("Alice")),
			wantSQL:  `INSERT INTO "users" ("id", "email", "name") VALUES (?, ?, ?)`,
			wantArgs: []any{int64(1), "alice@example.com", "Alice"},
		},
		"do nothing for target": {
			query: query.Values(id.Value(int64(1)), email.Value("alice@example.com"), name.Value("Alice")).
				OnConflict(email).DoNothing(),
			wantSQL: `INSERT INTO "users" ("id", "email", "name") VALUES (?, ?, ?) ` +
				`ON CONFLICT ("email") DO NOTHING`,
			wantArgs: []any{int64(1), "alice@example.com", "Alice"},
		},
		"do nothing for any conflict": {
			query: query.Values(id.Value(int64(1)), email.Value("alice@example.com"), name.Value("Alice")).
				OnConflict().DoNothing(),
			wantSQL: `INSERT INTO "users" ("id", "email", "name") VALUES (?, ?, ?) ` +
				`ON CONFLICT DO NOTHING`,
			wantArgs: []any{int64(1), "alice@example.com", "Alice"},
		},
		"update from excluded": {
			query: query.Values(id.Value(int64(1)), email.Value("alice@example.com"), name.Value("Alice")).
				OnConflict(email).DoUpdate(querysqlite.Excluded(name)),
			wantSQL: `INSERT INTO "users" ("id", "email", "name") VALUES (?, ?, ?) ` +
				`ON CONFLICT ("email") DO UPDATE SET "name" = EXCLUDED."name"`,
			wantArgs: []any{int64(1), "alice@example.com", "Alice"},
		},
		"update without target": {
			query: query.Values(id.Value(int64(1)), email.Value("alice@example.com"), name.Value("Alice")).
				OnConflict().DoUpdate(querysqlite.Excluded(name)),
			wantSQL: `INSERT INTO "users" ("id", "email", "name") VALUES (?, ?, ?) ` +
				`ON CONFLICT DO UPDATE SET "name" = EXCLUDED."name"`,
			wantArgs: []any{int64(1), "alice@example.com", "Alice"},
		},
		"explicit value after bulk rows": {
			query: query.Values(id.Value(int64(1)), email.Value("alice@example.com"), name.Value("Alice")).
				Values(id.Value(int64(2)), email.Value("bob@example.com"), name.Value("Bob")).
				OnConflict(email).DoUpdate(name.To("Existing")),
			wantSQL: `INSERT INTO "users" ("id", "email", "name") VALUES (?, ?, ?), (?, ?, ?) ` +
				`ON CONFLICT ("email") DO UPDATE SET "name" = ?`,
			wantArgs: []any{int64(1), "alice@example.com", "Alice", int64(2), "bob@example.com", "Bob", "Existing"},
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			statement := test.query.MustBuild()
			require.Equal(t, test.wantSQL, statement.SQL())
			require.Equal(t, test.wantArgs, statement.Args())
		})
	}
}

func TestOnConflictValidation(t *testing.T) {
	table := tiqq.NewSchemaInfo(tiqq.SQLite).Table("users")
	id := tiqq.RequiredNumericColumn[userScope, int64, int64, float64]("users", "id")
	name := tiqq.RequiredColumn[userScope, string]("users", "name")
	query := querysqlite.NewInsert(tiqq.NewInsert[userScope, tiqq.SQLiteMarker](
		table, []string{"id", "name"}, []string{"id", "name"}, [][]string{{"id"}},
	))
	tests := map[string]struct {
		query querysqlite.InsertQuery[userScope]
		want  string
	}{
		"target must be unique": {
			query: query.Values(id.Value(int64(1)), name.Value("Alice")).OnConflict(name).DoNothing(),
			want:  "tiqq: ON CONFLICT target does not match a primary key or unique constraint",
		},
		"update requires assignment": {
			query: query.Values(id.Value(int64(1)), name.Value("Alice")).OnConflict(id).DoUpdate(),
			want:  "tiqq: ON CONFLICT DO UPDATE requires at least one assignment",
		},
		"assignment must be unique": {
			query: query.Values(id.Value(int64(1)), name.Value("Alice")).
				OnConflict(id).DoUpdate(name.To("A"), querysqlite.Excluded(name)),
			want: "tiqq: ON CONFLICT assignment column users.name is specified more than once",
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			_, err := test.query.Build()
			require.EqualError(t, err, test.want)
		})
	}
}

func TestOnConflictExecutesOnSQLite(t *testing.T) {
	database, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = database.Close() })
	_, err = database.Exec(`CREATE TABLE users (id INTEGER PRIMARY KEY, email TEXT UNIQUE, name TEXT NOT NULL)`)
	require.NoError(t, err)
	_, err = database.Exec(`INSERT INTO users (id, email, name) VALUES (1, 'alice@example.com', 'Old')`)
	require.NoError(t, err)

	table := tiqq.NewSchemaInfo(tiqq.SQLite).Table("users")
	id := tiqq.RequiredNumericColumn[userScope, int64, int64, float64]("users", "id")
	email := tiqq.RequiredColumn[userScope, string]("users", "email")
	name := tiqq.RequiredColumn[userScope, string]("users", "name")
	query := querysqlite.NewInsert(tiqq.NewInsert[userScope, tiqq.SQLiteMarker](
		table, []string{"id", "email", "name"}, []string{"id", "email", "name"},
		[][]string{{"id"}, {"email"}},
	))
	statement := query.Values(
		id.Value(int64(2)), email.Value("alice@example.com"), name.Value("Updated"),
	).OnConflict(email).DoUpdate(querysqlite.Excluded(name)).MustBuild()

	_, err = database.Exec(statement.SQL(), statement.Args()...)
	require.NoError(t, err)
	var got string
	require.NoError(t, database.QueryRow(`SELECT name FROM users WHERE email = ?`, "alice@example.com").Scan(&got))
	require.Equal(t, "Updated", got)
}
