package tiqq_test

import (
	"testing"

	"github.com/eji/tiqq"
	. "github.com/eji/tiqq/example/schema"
	"github.com/stretchr/testify/require"
)

func TestDeleteBuild(t *testing.T) {
	tests := map[string]struct {
		build    func() tiqq.Statement
		wantSQL  string
		wantArgs []any
	}{
		"where is rendered": {
			build: func() tiqq.Statement {
				return UserTable.Delete().Where(UserTable.ID.Eq(int64(7))).MustBuild()
			},
			wantSQL:  `DELETE FROM "users" WHERE "users"."id" = $1`,
			wantArgs: []any{int64(7)},
		},
		"dynamic where calls preserve order": {
			build: func() tiqq.Statement {
				query := UserTable.Delete().Where(UserTable.ID.Gt(int64(1)))
				query = query.Where(UserTable.Name.Like("A%"))
				return query.MustBuild()
			},
			wantSQL:  `DELETE FROM "users" WHERE "users"."id" > $1 AND "users"."name" LIKE $2`,
			wantArgs: []any{int64(1), "A%"},
		},
		"all rows is explicit": {
			build: func() tiqq.Statement {
				return AuditLogTable.Delete().AllRows().MustBuild()
			},
			wantSQL:  `DELETE FROM "audit_logs"`,
			wantArgs: nil,
		},
		"alias is rendered": {
			build: func() tiqq.Statement {
				users := UserTable.As("target")
				return users.Delete().Where(users.ID.Eq(int64(7))).MustBuild()
			},
			wantSQL:  `DELETE FROM "users" AS "target" WHERE "target"."id" = $1`,
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

func TestDeleteBuildValidation(t *testing.T) {
	tests := map[string]struct {
		build func() error
		want  string
	}{
		"where or all rows is required": {
			build: func() error {
				_, err := UserTable.Delete().Build()
				return err
			},
			want: "tiqq: DELETE requires WHERE or AllRows",
		},
		"empty where is not authorization": {
			build: func() error {
				_, err := UserTable.Delete().Where().Build()
				return err
			},
			want: "tiqq: DELETE requires WHERE or AllRows",
		},
		"where must belong to delete table": {
			build: func() error {
				_, err := UserTable.Delete().Where(AddressTable.ID.Eq(int64(1))).Build()
				return err
			},
			want: "tiqq: WHERE column addresses.id is not in query scope",
		},
		"where rejects aggregate": {
			build: func() error {
				_, err := UserTable.Delete().Where(UserTable.ID.Count().Gt(int64(1))).Build()
				return err
			},
			want: "tiqq: WHERE does not accept aggregate COUNT(users.id)",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			require.EqualError(t, test.build(), test.want)
		})
	}
}

func TestDeleteMustBuildPanicsOnValidationError(t *testing.T) {
	require.PanicsWithError(t, "tiqq: DELETE requires WHERE or AllRows", func() {
		UserTable.Delete().MustBuild()
	})
}
