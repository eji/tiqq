package tiqq_test

import (
	"testing"

	"github.com/eji/tiqq"
	. "github.com/eji/tiqq/example/schema"
	"github.com/stretchr/testify/require"
)

func TestUpdateBuild(t *testing.T) {
	tests := map[string]struct {
		build    func() tiqq.Statement
		wantSQL  string
		wantArgs []any
	}{
		"dynamic set calls preserve order": {
			build: func() tiqq.Statement {
				query := UserTable.Update().Set(UserTable.Name.To("Alice"))
				query = query.Set(UserTable.ManagerID.To(int64(42)))
				return query.Where(UserTable.ID.Eq(int64(7))).MustBuild()
			},
			wantSQL:  `UPDATE "users" SET "name" = $1, "manager_id" = $2 WHERE "users"."id" = $3`,
			wantArgs: []any{"Alice", int64(42), int64(7)},
		},
		"multiple where calls preserve order": {
			build: func() tiqq.Statement {
				query := UserTable.Update().Set(UserTable.Name.To("Alice"))
				query = query.Where(UserTable.ID.Gt(int64(1)))
				return query.Where(UserTable.ID.Lt(int64(10))).MustBuild()
			},
			wantSQL:  `UPDATE "users" SET "name" = $1 WHERE "users"."id" > $2 AND "users"."id" < $3`,
			wantArgs: []any{"Alice", int64(1), int64(10)},
		},
		"all rows is explicit": {
			build: func() tiqq.Statement {
				return CompanyTable.Update().Set(CompanyTable.Name.To("Unknown")).AllRows().MustBuild()
			},
			wantSQL:  `UPDATE "companies" SET "name" = $1`,
			wantArgs: []any{"Unknown"},
		},
		"alias is rendered": {
			build: func() tiqq.Statement {
				users := UserTable.As("target")
				return users.Update().Set(users.Name.To("Alice")).Where(users.ID.Eq(int64(7))).MustBuild()
			},
			wantSQL:  `UPDATE "users" AS "target" SET "name" = $1 WHERE "target"."id" = $2`,
			wantArgs: []any{"Alice", int64(7)},
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

func TestUpdateBuildValidation(t *testing.T) {
	tests := map[string]struct {
		build func() error
		want  string
	}{
		"set is required": {
			build: func() error {
				_, err := UserTable.Update().Where(UserTable.ID.Eq(int64(1))).Build()
				return err
			},
			want: "tiqq: UPDATE requires at least one SET assignment",
		},
		"where or all rows is required": {
			build: func() error {
				_, err := UserTable.Update().Set(UserTable.Name.To("Alice")).Build()
				return err
			},
			want: "tiqq: UPDATE requires WHERE or AllRows",
		},
		"where must belong to update table": {
			build: func() error {
				_, err := UserTable.Update().
					Set(UserTable.Name.To("Alice")).
					Where(AddressTable.ID.Eq(int64(1))).
					Build()
				return err
			},
			want: "tiqq: WHERE column addresses.id is not in query scope",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			require.EqualError(t, test.build(), test.want)
		})
	}
}

func TestUpdateMustBuildPanicsOnValidationError(t *testing.T) {
	require.PanicsWithError(t, "tiqq: UPDATE requires WHERE or AllRows", func() {
		UserTable.Update().Set(UserTable.Name.To("Alice")).MustBuild()
	})
}
