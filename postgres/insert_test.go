package postgres_test

import (
	"testing"

	. "github.com/eji/tiqq/example/schema"
	"github.com/eji/tiqq/postgres"
	"github.com/stretchr/testify/require"
)

func TestOnConflictBuild(t *testing.T) {
	tests := map[string]struct {
		build    func() (string, []any)
		wantSQL  string
		wantArgs []any
	}{
		"do nothing": {
			build: func() (string, []any) {
				statement := UserTable.Insert().
					Values(UserTable.ID.Value(int64(1)), UserTable.Name.Value("Alice")).
					OnConflict(UserTable.ID).
					DoNothing().
					MustBuild()
				return statement.SQL(), statement.Args()
			},
			wantSQL:  `INSERT INTO "users" ("id", "name") VALUES ($1, $2) ON CONFLICT ("id") DO NOTHING`,
			wantArgs: []any{int64(1), "Alice"},
		},
		"do nothing for any conflict": {
			build: func() (string, []any) {
				statement := UserTable.Insert().
					Values(UserTable.ID.Value(int64(1)), UserTable.Name.Value("Alice")).
					OnConflict().
					DoNothing().
					MustBuild()
				return statement.SQL(), statement.Args()
			},
			wantSQL:  `INSERT INTO "users" ("id", "name") VALUES ($1, $2) ON CONFLICT DO NOTHING`,
			wantArgs: []any{int64(1), "Alice"},
		},
		"update from excluded": {
			build: func() (string, []any) {
				statement := UserTable.Insert().
					Values(UserTable.ID.Value(int64(1)), UserTable.Name.Value("Alice")).
					OnConflict(UserTable.ID).
					DoUpdate(postgres.Excluded(UserTable.Name)).
					MustBuild()
				return statement.SQL(), statement.Args()
			},
			wantSQL:  `INSERT INTO "users" ("id", "name") VALUES ($1, $2) ON CONFLICT ("id") DO UPDATE SET "name" = EXCLUDED."name"`,
			wantArgs: []any{int64(1), "Alice"},
		},
		"update to explicit value after bulk rows": {
			build: func() (string, []any) {
				statement := UserTable.Insert().
					Values(UserTable.ID.Value(int64(1)), UserTable.Name.Value("Alice")).
					Values(UserTable.ID.Value(int64(2)), UserTable.Name.Value("Bob")).
					OnConflict(UserTable.ID).
					DoUpdate(UserTable.Name.To("Existing")).
					MustBuild()
				return statement.SQL(), statement.Args()
			},
			wantSQL:  `INSERT INTO "users" ("id", "name") VALUES ($1, $2), ($3, $4) ON CONFLICT ("id") DO UPDATE SET "name" = $5`,
			wantArgs: []any{int64(1), "Alice", int64(2), "Bob", "Existing"},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			gotSQL, gotArgs := test.build()
			require.Equal(t, test.wantSQL, gotSQL)
			require.Equal(t, test.wantArgs, gotArgs)
		})
	}
}

func TestOnConflictValidation(t *testing.T) {
	tests := map[string]struct {
		build func() error
		want  string
	}{
		"target must be unique": {
			build: func() error {
				_, err := UserTable.Insert().
					Values(UserTable.ID.Value(int64(1)), UserTable.Name.Value("Alice")).
					OnConflict(UserTable.Name).
					DoNothing().
					Build()
				return err
			},
			want: "tiqq: ON CONFLICT target does not match a primary key or unique constraint",
		},
		"update requires assignment": {
			build: func() error {
				_, err := UserTable.Insert().
					Values(UserTable.ID.Value(int64(1)), UserTable.Name.Value("Alice")).
					OnConflict(UserTable.ID).
					DoUpdate().
					Build()
				return err
			},
			want: "tiqq: ON CONFLICT DO UPDATE requires at least one assignment",
		},
		"update requires target": {
			build: func() error {
				_, err := UserTable.Insert().
					Values(UserTable.ID.Value(int64(1)), UserTable.Name.Value("Alice")).
					OnConflict().
					DoUpdate(postgres.Excluded(UserTable.Name)).
					Build()
				return err
			},
			want: "tiqq: ON CONFLICT DO UPDATE requires a conflict target",
		},
		"assignment must be unique": {
			build: func() error {
				_, err := UserTable.Insert().
					Values(UserTable.ID.Value(int64(1)), UserTable.Name.Value("Alice")).
					OnConflict(UserTable.ID).
					DoUpdate(UserTable.Name.To("A"), postgres.Excluded(UserTable.Name)).
					Build()
				return err
			},
			want: "tiqq: ON CONFLICT assignment column users.name is specified more than once",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			require.EqualError(t, test.build(), test.want)
		})
	}
}
