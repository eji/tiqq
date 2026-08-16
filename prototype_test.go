package tiqq_test

import (
	"database/sql"
	"testing"

	"github.com/eji/tiqq"
	. "github.com/eji/tiqq/example/schema"
	"github.com/stretchr/testify/require"
)

func prototypeJoin() UserAddressJoin {
	return UserTable.LeftJoin(
		AddressTable,
		tiqq.On(UserTable.ID, AddressTable.UserID),
	)
}

func TestBuild(t *testing.T) {
	j := prototypeJoin()
	tests := map[string]struct {
		predicates []tiqq.Predicate[UserAddressScope]
		wantSQL    string
		wantArgs   []any
	}{
		"without predicates": {
			wantSQL:  `SELECT "users"."id", "users"."name", "addresses"."address" FROM "users" LEFT JOIN "addresses" ON "users"."id" = "addresses"."user_id"`,
			wantArgs: nil,
		},
		"with predicates": {
			predicates: []tiqq.Predicate[UserAddressScope]{
				j.Left().ID.Eq(int64(100)),
				j.Right().Address.Like("Tokyo%"),
			},
			wantSQL:  `SELECT "users"."id", "users"."name", "addresses"."address" FROM "users" LEFT JOIN "addresses" ON "users"."id" = "addresses"."user_id" WHERE "users"."id" = $1 AND "addresses"."address" LIKE $2`,
			wantArgs: []any{int64(100), "Tokyo%"},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			stmt := j.
				Where(test.predicates...).
				Select(j.Left().ID, j.Left().Name, j.Right().Address).
				Build()

			require.Equal(t, test.wantSQL, stmt.SQL())
			require.Equal(t, test.wantArgs, stmt.Args())
		})
	}
}

func TestInnerJoin(t *testing.T) {
	j := UserTable.InnerJoin(
		AddressTable,
		tiqq.On(UserTable.ID, AddressTable.UserID),
	)
	stmt := j.
		Where(j.Right().Address.Like("Tokyo%")).
		Select(j.Left().ID, j.Right().Address).
		Build()
	row, err := tiqq.NewRow(stmt, int64(100), "Tokyo")

	require.Equal(
		t,
		`SELECT "users"."id", "addresses"."address" FROM "users" INNER JOIN "addresses" ON "users"."id" = "addresses"."user_id" WHERE "addresses"."address" LIKE $1`,
		stmt.SQL(),
	)
	require.Equal(t, []any{"Tokyo%"}, stmt.Args())
	require.NoError(t, err)
	require.Equal(t, int64(100), row.Get(j.Left().ID))
	require.Equal(t, "Tokyo", row.Get(j.Right().Address))
}

func TestPredicateOperators(t *testing.T) {
	j := prototypeJoin()
	tests := map[string]struct {
		predicate tiqq.Predicate[UserAddressScope]
		wantSQL   string
		wantArg   any
	}{
		"equal":              {predicate: j.Left().ID.Eq(10), wantSQL: `"users"."id" = $1`, wantArg: int64(10)},
		"not equal":          {predicate: j.Left().ID.Ne(10), wantSQL: `"users"."id" <> $1`, wantArg: int64(10)},
		"greater than":       {predicate: j.Left().ID.Gt(10), wantSQL: `"users"."id" > $1`, wantArg: int64(10)},
		"greater or equal":   {predicate: j.Right().ID.Gte(10), wantSQL: `"addresses"."id" >= $1`, wantArg: int64(10)},
		"less than":          {predicate: j.Right().ID.Lt(10), wantSQL: `"addresses"."id" < $1`, wantArg: int64(10)},
		"less than or equal": {predicate: j.Right().ID.Lte(10), wantSQL: `"addresses"."id" <= $1`, wantArg: int64(10)},
		"like":               {predicate: j.Left().Name.Like("A%"), wantSQL: `"users"."name" LIKE $1`, wantArg: "A%"},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			stmt := j.Select(j.Left().ID).Where(test.predicate).Build()
			require.Contains(t, stmt.SQL(), " WHERE "+test.wantSQL)
			require.Equal(t, []any{test.wantArg}, stmt.Args())
		})
	}
}

func TestTypedRowGet(t *testing.T) {
	j := prototypeJoin()
	stmt := j.Select(j.Left().ID, j.Left().Name, j.Right().Address).Build()
	wantAddress := sql.Null[string]{V: "Tokyo", Valid: true}
	row, err := tiqq.NewRow(stmt, int64(100), "Alice", wantAddress)

	require.NoError(t, err)
	require.Equal(t, int64(100), row.Get(j.Left().ID))
	require.Equal(t, "Alice", row.Get(j.Left().Name))
	require.Equal(t, wantAddress, row.Get(j.Right().Address))
}

func TestRowValidation(t *testing.T) {
	j := prototypeJoin()
	tests := map[string]struct {
		run       func()
		wantPanic string
	}{
		"column is absent from projection": {
			run: func() {
				stmt := j.Select(j.Left().ID).Build()
				row, err := tiqq.NewRow(stmt, int64(1))
				require.NoError(t, err)
				row.Get(j.Left().Name)
			},
			wantPanic: "tiqq: column users.name is not in the projection",
		},
		"driver value has wrong type": {
			run: func() {
				stmt := j.Select(j.Left().ID).Build()
				row, err := tiqq.NewRow(stmt, "not an int64")
				require.NoError(t, err)
				row.Get(j.Left().ID)
			},
			wantPanic: "tiqq: column users.id: cannot use string as int64",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			require.PanicsWithValue(t, test.wantPanic, test.run)
		})
	}
}

func TestNewRowRejectsProjectionLengthMismatch(t *testing.T) {
	j := prototypeJoin()
	stmt := j.Select(j.Left().ID, j.Left().Name).Build()
	_, err := tiqq.NewRow(stmt, int64(1))

	require.EqualError(t, err, "tiqq: got 1 values for 2 projected columns")
}

func TestBuildRequiresProjection(t *testing.T) {
	j := prototypeJoin()
	require.PanicsWithValue(
		t,
		"tiqq: SELECT requires at least one column",
		func() { j.Where(j.Left().ID.Eq(1)).Build() },
	)
}

func TestStatementArgsReturnsCopy(t *testing.T) {
	j := prototypeJoin()
	stmt := j.Select(j.Left().ID).Where(j.Left().ID.Eq(1)).Build()
	args := stmt.Args()
	args[0] = int64(999)

	require.Equal(t, []any{int64(1)}, stmt.Args())
}

func TestQueryIsImmutable(t *testing.T) {
	j := prototypeJoin()
	base := j.Select(j.Left().ID)
	filtered := base.Where(j.Left().ID.Eq(1))

	require.NotContains(t, base.Build().SQL(), " WHERE ")
	require.Contains(t, filtered.Build().SQL(), ` WHERE "users"."id" = $1`)
}
