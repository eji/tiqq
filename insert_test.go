package tiqq_test

import (
	"testing"

	"github.com/eji/tiqq"
	. "github.com/eji/tiqq/example/schema"
	"github.com/stretchr/testify/require"
)

func TestInsertBuild(t *testing.T) {
	tests := map[string]struct {
		build    func() tiqq.Statement
		wantSQL  string
		wantArgs []any
	}{
		"single row": {
			build: func() tiqq.Statement {
				return UserTable.Insert(
					UserTable.ID.Value(int64(1)),
					UserTable.Name.Value("Alice"),
				).MustBuild()
			},
			wantSQL:  `INSERT INTO "users" ("id", "name") VALUES ($1, $2)`,
			wantArgs: []any{int64(1), "Alice"},
		},
		"multiple rows": {
			build: func() tiqq.Statement {
				return UserTable.Insert(
					UserTable.ID.Value(int64(1)),
					UserTable.Name.Value("Alice"),
				).Values(
					UserTable.ID.Value(int64(2)),
					UserTable.Name.Value("Bob"),
				).MustBuild()
			},
			wantSQL:  `INSERT INTO "users" ("id", "name") VALUES ($1, $2), ($3, $4)`,
			wantArgs: []any{int64(1), "Alice", int64(2), "Bob"},
		},
		"nullable column accepts base value": {
			build: func() tiqq.Statement {
				return UserTable.Insert(
					UserTable.ID.Value(int64(1)),
					UserTable.Name.Value("Alice"),
					UserTable.ManagerID.Value(int64(9)),
				).MustBuild()
			},
			wantSQL:  `INSERT INTO "users" ("id", "name", "manager_id") VALUES ($1, $2, $3)`,
			wantArgs: []any{int64(1), "Alice", int64(9)},
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

func TestInsertBuildValidation(t *testing.T) {
	tests := map[string]struct {
		build func() error
		want  string
	}{
		"value is required": {
			build: func() error {
				_, err := UserTable.Insert().Build()
				return err
			},
			want: "tiqq: INSERT requires at least one value",
		},
		"required column is missing": {
			build: func() error {
				_, err := UserTable.Insert(UserTable.ID.Value(int64(1))).Build()
				return err
			},
			want: "tiqq: INSERT requires column name",
		},
		"column is duplicated": {
			build: func() error {
				_, err := UserTable.Insert(
					UserTable.ID.Value(int64(1)),
					UserTable.ID.Value(int64(2)),
					UserTable.Name.Value("Alice"),
				).Build()
				return err
			},
			want: "tiqq: INSERT column users.id is specified more than once",
		},
		"bulk column count must match": {
			build: func() error {
				_, err := UserTable.Insert(
					UserTable.ID.Value(int64(1)), UserTable.Name.Value("Alice"),
				).Values(UserTable.ID.Value(int64(2))).Build()
				return err
			},
			want: "tiqq: INSERT row 2 columns do not match the first row",
		},
		"bulk column order must match": {
			build: func() error {
				_, err := UserTable.Insert(
					UserTable.ID.Value(int64(1)), UserTable.Name.Value("Alice"),
				).Values(
					UserTable.Name.Value("Bob"), UserTable.ID.Value(int64(2)),
				).Build()
				return err
			},
			want: "tiqq: INSERT row 2 columns do not match the first row",
		},
		"generated column is rejected": {
			build: func() error {
				query := tiqq.NewInsert[UserScope](
					tiqq.NewTableRef("users"), []string{"name"}, []string{"name"},
					UserTable.ID.Value(int64(1)), UserTable.Name.Value("Alice"),
				)
				_, err := query.Build()
				return err
			},
			want: "tiqq: column users.id is not insertable",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			require.EqualError(t, test.build(), test.want)
		})
	}
}

func TestInsertMustBuildPanicsOnValidationError(t *testing.T) {
	require.PanicsWithError(t, "tiqq: INSERT requires at least one value", func() {
		UserTable.Insert().MustBuild()
	})
}
