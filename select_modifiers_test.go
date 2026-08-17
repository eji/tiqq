package tiqq_test

import (
	"testing"

	. "github.com/eji/tiqq/example/schema"
	"github.com/stretchr/testify/require"
)

func TestSelectModifiersBuild(t *testing.T) {
	count := AddressTable.ID.Count()
	tests := map[string]struct {
		build    func() (string, []any)
		wantSQL  string
		wantArgs []any
	}{
		"distinct ordered page": {
			build: func() (string, []any) {
				statement := UserTable.Where(UserTable.Name.Like("A%")).
					Select(UserTable.ID, UserTable.Name).
					Distinct().
					OrderBy(UserTable.Name.Asc(), UserTable.ID.Desc()).
					Limit(20).
					Offset(40).
					MustBuild()
				return statement.SQL(), statement.Args()
			},
			wantSQL:  `SELECT DISTINCT "users"."id", "users"."name" FROM "users" WHERE "users"."name" LIKE $1 ORDER BY "users"."name" ASC, "users"."id" DESC LIMIT $2 OFFSET $3`,
			wantArgs: []any{"A%", int64(20), int64(40)},
		},
		"ordered aggregate": {
			build: func() (string, []any) {
				statement := AddressTable.GroupBy(AddressTable.UserID).
					Select(AddressTable.UserID, count).
					OrderBy(count.Desc()).
					MustBuild()
				return statement.SQL(), statement.Args()
			},
			wantSQL:  `SELECT "addresses"."user_id", COUNT("addresses"."id") FROM "addresses" GROUP BY "addresses"."user_id" ORDER BY COUNT("addresses"."id") DESC`,
			wantArgs: nil,
		},
		"zero limit": {
			build: func() (string, []any) {
				statement := UserTable.Select(UserTable.ID).Limit(0).MustBuild()
				return statement.SQL(), statement.Args()
			},
			wantSQL:  `SELECT "users"."id" FROM "users" LIMIT $1`,
			wantArgs: []any{int64(0)},
		},
		"postgres offset without limit": {
			build: func() (string, []any) {
				statement := UserTable.Select(UserTable.ID).Offset(5).MustBuild()
				return statement.SQL(), statement.Args()
			},
			wantSQL:  `SELECT "users"."id" FROM "users" OFFSET $1`,
			wantArgs: []any{int64(5)},
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

func TestSelectModifiersValidation(t *testing.T) {
	tests := map[string]struct {
		build func() error
		want  string
	}{
		"negative limit": {
			build: func() error {
				_, err := UserTable.Select(UserTable.ID).Limit(-1).Build()
				return err
			},
			want: "tiqq: LIMIT must not be negative",
		},
		"negative offset": {
			build: func() error {
				_, err := UserTable.Select(UserTable.ID).Limit(1).Offset(-1).Build()
				return err
			},
			want: "tiqq: OFFSET must not be negative",
		},
		"order column must belong to query": {
			build: func() error {
				_, err := UserTable.Select(UserTable.ID).OrderBy(AddressTable.ID.Asc()).Build()
				return err
			},
			want: "tiqq: ORDER BY column addresses.id is not in query scope",
		},
		"distinct order column must be selected": {
			build: func() error {
				_, err := UserTable.Select(UserTable.ID).Distinct().OrderBy(UserTable.Name.Asc()).Build()
				return err
			},
			want: "tiqq: DISTINCT ORDER BY column users.name must appear in SELECT",
		},
		"aggregate order requires grouped projection": {
			build: func() error {
				_, err := AddressTable.Select(AddressTable.UserID).OrderBy(AddressTable.ID.Count().Desc()).Build()
				return err
			},
			want: "tiqq: SELECT column addresses.user_id must appear in GROUP BY",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			require.EqualError(t, test.build(), test.want)
		})
	}
}
