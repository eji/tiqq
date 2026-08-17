package tiqq_test

import (
	"database/sql"
	"testing"

	"github.com/eji/tiqq"
	. "github.com/eji/tiqq/example/schema"
	"github.com/stretchr/testify/require"
)

func prototypeJoin() tiqq.Joined[UserTableDef, NullableAddressTableDef, NullableUserTableDef, NullableAddressTableDef] {
	return UserTable.LeftJoin(AddressTable).On(tiqq.Eq(UserTable.ID, AddressTable.UserID))
}

func TestBuild(t *testing.T) {
	j := UserTable.LeftJoin(AddressTable).On(tiqq.Eq(UserTable.ID, AddressTable.UserID))
	tests := map[string]struct {
		predicates []tiqq.Predicate
		wantSQL    string
		wantArgs   []any
	}{
		"without predicates": {
			wantSQL:  `SELECT "users"."id", "users"."name", "addresses"."address" FROM "users" LEFT JOIN "addresses" ON "users"."id" = "addresses"."user_id"`,
			wantArgs: nil,
		},
		"with predicates": {
			predicates: []tiqq.Predicate{
				j.Left().ID.Eq(int64(100)),
				j.Right().Address.Like("Tokyo%"),
			},
			wantSQL:  `SELECT "users"."id", "users"."name", "addresses"."address" FROM "users" LEFT JOIN "addresses" ON "users"."id" = "addresses"."user_id" WHERE "users"."id" = $1 AND "addresses"."address" LIKE $2`,
			wantArgs: []any{int64(100), "Tokyo%"},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			stmt, err := j.
				Where(test.predicates...).
				Select(j.Left().ID, j.Left().Name, j.Right().Address).
				Build()

			require.NoError(t, err)
			require.Equal(t, test.wantSQL, stmt.SQL())
			require.Equal(t, test.wantArgs, stmt.Args())
		})
	}
}

func TestInnerJoin(t *testing.T) {
	j := UserTable.InnerJoin(
		AddressTable,
	).On(tiqq.Eq(UserTable.ID, AddressTable.UserID))
	stmt := j.
		Where(j.Right().Address.Like("Tokyo%")).
		Select(j.Left().ID, j.Right().Address).
		MustBuild()
	row, err := tiqq.NewRow(stmt, int64(100), "Tokyo")

	require.Equal(
		t,
		`SELECT "users"."id", "addresses"."address" FROM "users" INNER JOIN "addresses" ON "users"."id" = "addresses"."user_id" WHERE "addresses"."address" LIKE $1`,
		stmt.SQL(),
	)
	require.Equal(t, []any{"Tokyo%"}, stmt.Args())
	require.NoError(t, err)
	require.Equal(t, int64(100), row.MustGet(j.Left().ID))
	require.Equal(t, "Tokyo", row.MustGet(j.Right().Address))
}

func TestLeftJoinInfersForeignKey(t *testing.T) {
	j := UserTable.LeftJoin(AddressTable)
	stmt := j.Select(j.Left().ID, j.Right().Address).MustBuild()

	require.Equal(
		t,
		`SELECT "users"."id", "addresses"."address" FROM "users" LEFT JOIN "addresses" ON "addresses"."user_id" = "users"."id"`,
		stmt.SQL(),
	)
}

func TestLeftJoinExplicitOnPredicates(t *testing.T) {
	j := UserTable.LeftJoin(AuditLogTable).On(
		tiqq.Eq(UserTable.ID, AuditLogTable.ActorID),
		AuditLogTable.Active.Eq(true),
	)
	stmt := j.
		Where(j.Left().ID.Eq(100)).
		Select(j.Left().ID, j.Right().Message).
		MustBuild()

	require.Equal(
		t,
		`SELECT "users"."id", "audit_logs"."message" FROM "users" LEFT JOIN "audit_logs" ON "users"."id" = "audit_logs"."actor_id" AND "audit_logs"."active" = $1 WHERE "users"."id" = $2`,
		stmt.SQL(),
	)
	require.Equal(t, []any{true, int64(100)}, stmt.Args())
}

func TestJoinBuildValidation(t *testing.T) {
	tests := map[string]struct {
		build func() error
		want  string
	}{
		"missing foreign key requires ON": {
			build: func() error {
				joined := UserTable.LeftJoin(AuditLogTable)
				_, err := joined.Select(joined.Left().ID).Build()
				return err
			},
			want: "tiqq: JOIN requires ON because no foreign key matches",
		},
		"ON rejects unrelated table": {
			build: func() error {
				joined := UserTable.LeftJoin(AuditLogTable).
					On(tiqq.Or(
						tiqq.Eq(CompanyTable.ID, AuditLogTable.ActorID),
						AuditLogTable.Active.Eq(true),
					))
				_, err := joined.Select(joined.Left().ID).Build()
				return err
			},
			want: "tiqq: ON column companies.id is not in query scope",
		},
		"SELECT rejects unrelated table": {
			build: func() error {
				joined := UserTable.LeftJoin(AddressTable)
				_, err := joined.Select(CompanyTable.Name).Build()
				return err
			},
			want: "tiqq: SELECT column companies.name is not in query scope",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			require.EqualError(t, test.build(), test.want)
		})
	}
}

func TestMultiStageLeftJoin(t *testing.T) {
	userAddress := UserTable.LeftJoin(AddressTable)
	joined := tiqq.LeftJoin(userAddress, CompanyTable)
	stmt := joined.
		Where(joined.Left().Left().ID.Eq(100)).
		Select(
			joined.Left().Left().ID,
			joined.Left().Right().Address,
			joined.Right().Name,
		).
		MustBuild()
	wantAddress := sql.Null[string]{V: "Tokyo", Valid: true}
	wantCompany := sql.Null[string]{V: "Acme", Valid: true}
	row, err := tiqq.NewRow(stmt, int64(100), wantAddress, wantCompany)

	require.Equal(
		t,
		`SELECT "users"."id", "addresses"."address", "companies"."name" FROM "users" LEFT JOIN "addresses" ON "addresses"."user_id" = "users"."id" LEFT JOIN "companies" ON "addresses"."company_id" = "companies"."id" WHERE "users"."id" = $1`,
		stmt.SQL(),
	)
	require.Equal(t, []any{int64(100)}, stmt.Args())
	require.NoError(t, err)
	require.Equal(t, int64(100), row.MustGet(joined.Left().Left().ID))
	require.Equal(t, wantAddress, row.MustGet(joined.Left().Right().Address))
	require.Equal(t, wantCompany, row.MustGet(joined.Right().Name))
}

func TestMultiStageJoinWithExplicitOn(t *testing.T) {
	userAddress := UserTable.LeftJoin(AddressTable)
	userAddressCompany := tiqq.LeftJoin(userAddress, CompanyTable)
	joined := tiqq.LeftJoin(userAddressCompany, AuditLogTable).On(
		tiqq.Eq(userAddressCompany.Left().Left().ID, AuditLogTable.ActorID),
		AuditLogTable.Active.Eq(true),
	)
	stmt := joined.Select(
		joined.Left().Left().Left().ID,
		joined.Right().Message,
	).MustBuild()

	require.Contains(
		t,
		stmt.SQL(),
		`LEFT JOIN "audit_logs" ON "users"."id" = "audit_logs"."actor_id" AND "audit_logs"."active" = $1`,
	)
	require.Equal(t, []any{true}, stmt.Args())
}

func TestMultiStageInnerJoin(t *testing.T) {
	userAddress := UserTable.LeftJoin(AddressTable)
	joined := tiqq.InnerJoin(userAddress, CompanyTable)
	stmt := joined.Select(joined.Left().Right().Address, joined.Right().Name).MustBuild()
	wantAddress := sql.Null[string]{V: "Tokyo", Valid: true}
	row, err := tiqq.NewRow(stmt, wantAddress, "Acme")

	require.Contains(t, stmt.SQL(), `INNER JOIN "companies" ON "addresses"."company_id" = "companies"."id"`)
	require.NoError(t, err)
	require.Equal(t, wantAddress, row.MustGet(joined.Left().Right().Address))
	require.Equal(t, "Acme", row.MustGet(joined.Right().Name))
}

func TestSelfJoinWithAliases(t *testing.T) {
	employee := UserTable.As("employee")
	manager := UserTable.As("manager")
	j := employee.LeftJoin(manager).On(tiqq.Eq(employee.ManagerID, manager.ID))
	stmt := j.
		Where(j.Left().Name.Like("A%")).
		Select(j.Left().ID, j.Left().Name, j.Right().Name).
		MustBuild()
	wantManager := sql.Null[string]{V: "Bob", Valid: true}
	row, err := tiqq.NewRow(stmt, int64(1), "Alice", wantManager)

	require.Equal(
		t,
		`SELECT "employee"."id", "employee"."name", "manager"."name" FROM "users" AS "employee" LEFT JOIN "users" AS "manager" ON "employee"."manager_id" = "manager"."id" WHERE "employee"."name" LIKE $1`,
		stmt.SQL(),
	)
	require.Equal(t, []any{"A%"}, stmt.Args())
	require.NoError(t, err)
	require.Equal(t, int64(1), row.MustGet(j.Left().ID))
	require.Equal(t, "Alice", row.MustGet(j.Left().Name))
	require.Equal(t, wantManager, row.MustGet(j.Right().Name))
}

func TestEmptyAliasPanics(t *testing.T) {
	require.PanicsWithValue(t, "tiqq: table alias must not be empty", func() { UserTable.As("") })
}

func TestDuplicateAliasBuildValidation(t *testing.T) {
	left := UserTable.As("user")
	right := UserTable.As("user")
	_, err := left.LeftJoin(right).Select(left.ID).Build()

	require.EqualError(t, err, "tiqq: table aliases must be distinct")
}

func TestPredicateOperators(t *testing.T) {
	j := UserTable.LeftJoin(AddressTable).On(tiqq.Eq(UserTable.ID, AddressTable.UserID))
	tests := map[string]struct {
		predicate tiqq.Predicate
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
			stmt := j.Select(j.Left().ID).Where(test.predicate).MustBuild()
			require.Contains(t, stmt.SQL(), " WHERE "+test.wantSQL)
			require.Equal(t, []any{test.wantArg}, stmt.Args())
		})
	}
}

func TestPredicateExpressions(t *testing.T) {
	j := UserTable.LeftJoin(AddressTable)
	tests := map[string]struct {
		predicate tiqq.Predicate
		wantSQL   string
		wantArgs  []any
	}{
		"and": {
			predicate: tiqq.And(j.Left().ID.Gt(10), j.Left().ID.Lt(20)),
			wantSQL:   `("users"."id" > $1 AND "users"."id" < $2)`,
			wantArgs:  []any{int64(10), int64(20)},
		},
		"or": {
			predicate: tiqq.Or(j.Left().Name.Eq("Alice"), j.Left().Name.Eq("Bob")),
			wantSQL:   `("users"."name" = $1 OR "users"."name" = $2)`,
			wantArgs:  []any{"Alice", "Bob"},
		},
		"not": {
			predicate: tiqq.Not(j.Left().Name.Like("test%")),
			wantSQL:   `NOT ("users"."name" LIKE $1)`,
			wantArgs:  []any{"test%"},
		},
		"in": {
			predicate: j.Left().ID.In(1, 2, 3),
			wantSQL:   `"users"."id" IN ($1, $2, $3)`,
			wantArgs:  []any{int64(1), int64(2), int64(3)},
		},
		"not in": {
			predicate: j.Left().ID.NotIn(4, 5),
			wantSQL:   `"users"."id" NOT IN ($1, $2)`,
			wantArgs:  []any{int64(4), int64(5)},
		},
		"is null": {
			predicate: j.Right().Address.IsNull(),
			wantSQL:   `"addresses"."address" IS NULL`,
		},
		"is not null": {
			predicate: j.Right().Address.IsNotNull(),
			wantSQL:   `"addresses"."address" IS NOT NULL`,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			stmt := j.Select(j.Left().ID).Where(test.predicate).MustBuild()
			require.Contains(t, stmt.SQL(), " WHERE "+test.wantSQL)
			require.Equal(t, test.wantArgs, stmt.Args())
		})
	}
}

func TestColumnComparisonOperators(t *testing.T) {
	tests := map[string]struct {
		predicate tiqq.Predicate
		wantSQL   string
	}{
		"equal":            {predicate: tiqq.Eq(UserTable.ID, AddressTable.UserID), wantSQL: `"users"."id" = "addresses"."user_id"`},
		"not equal":        {predicate: tiqq.Ne(UserTable.ID, AddressTable.UserID), wantSQL: `"users"."id" <> "addresses"."user_id"`},
		"less than":        {predicate: tiqq.Lt(UserTable.ID, AddressTable.UserID), wantSQL: `"users"."id" < "addresses"."user_id"`},
		"less or equal":    {predicate: tiqq.Lte(UserTable.ID, AddressTable.UserID), wantSQL: `"users"."id" <= "addresses"."user_id"`},
		"greater than":     {predicate: tiqq.Gt(UserTable.ID, AddressTable.UserID), wantSQL: `"users"."id" > "addresses"."user_id"`},
		"greater or equal": {predicate: tiqq.Gte(UserTable.ID, AddressTable.UserID), wantSQL: `"users"."id" >= "addresses"."user_id"`},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			joined := UserTable.LeftJoin(AddressTable).On(test.predicate)
			stmt := joined.Select(joined.Left().ID).MustBuild()
			require.Contains(t, stmt.SQL(), " ON "+test.wantSQL)
		})
	}
}

func TestPredicateExpressionsInOn(t *testing.T) {
	j := UserTable.LeftJoin(AuditLogTable).On(
		tiqq.Eq(UserTable.ID, AuditLogTable.ActorID),
		tiqq.Or(
			AuditLogTable.Active.Eq(true),
			AuditLogTable.Message.Like("security:%"),
		),
	)
	stmt := j.Select(j.Left().ID).Where(j.Left().ID.In(10, 20)).MustBuild()

	require.Contains(
		t,
		stmt.SQL(),
		`ON "users"."id" = "audit_logs"."actor_id" AND ("audit_logs"."active" = $1 OR "audit_logs"."message" LIKE $2) WHERE "users"."id" IN ($3, $4)`,
	)
	require.Equal(t, []any{true, "security:%", int64(10), int64(20)}, stmt.Args())
}

func TestPredicateExpressionValidation(t *testing.T) {
	tests := map[string]struct {
		build     func()
		wantPanic string
	}{
		"empty AND": {
			build:     func() { tiqq.And() },
			wantPanic: "tiqq: AND requires at least one predicate",
		},
		"empty OR": {
			build:     func() { tiqq.Or() },
			wantPanic: "tiqq: OR requires at least one predicate",
		},
		"empty IN": {
			build:     func() { UserTable.ID.In() },
			wantPanic: "tiqq: IN requires at least one value",
		},
		"empty NOT IN": {
			build:     func() { UserTable.ID.NotIn() },
			wantPanic: "tiqq: NOT IN requires at least one value",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			require.PanicsWithValue(t, test.wantPanic, test.build)
		})
	}
}

func TestTypedRowGet(t *testing.T) {
	j := UserTable.LeftJoin(AddressTable).On(tiqq.Eq(UserTable.ID, AddressTable.UserID))
	stmt := j.Select(j.Left().ID, j.Left().Name, j.Right().Address).MustBuild()
	wantAddress := sql.Null[string]{V: "Tokyo", Valid: true}
	row, err := tiqq.NewRow(stmt, int64(100), "Alice", wantAddress)

	require.NoError(t, err)
	require.Equal(t, int64(100), row.MustGet(j.Left().ID))
	require.Equal(t, "Alice", row.MustGet(j.Left().Name))
	require.Equal(t, wantAddress, row.MustGet(j.Right().Address))
}

func TestRowValidation(t *testing.T) {
	j := UserTable.LeftJoin(AddressTable).On(tiqq.Eq(UserTable.ID, AddressTable.UserID))
	tests := map[string]struct {
		get  func() error
		want string
	}{
		"column is absent from projection": {
			get: func() error {
				stmt := j.Select(j.Left().ID).MustBuild()
				row, err := tiqq.NewRow(stmt, int64(1))
				require.NoError(t, err)
				_, err = row.Get(j.Left().Name)
				return err
			},
			want: "tiqq: column users.name is not in the projection",
		},
		"driver value has wrong type": {
			get: func() error {
				stmt := j.Select(j.Left().ID).MustBuild()
				row, err := tiqq.NewRow(stmt, "not an int64")
				require.NoError(t, err)
				_, err = row.Get(j.Left().ID)
				return err
			},
			want: "tiqq: column users.id: cannot use string as int64",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			require.EqualError(t, test.get(), test.want)
		})
	}
}

func TestNewRowRejectsProjectionLengthMismatch(t *testing.T) {
	j := UserTable.LeftJoin(AddressTable).On(tiqq.Eq(UserTable.ID, AddressTable.UserID))
	stmt := j.Select(j.Left().ID, j.Left().Name).MustBuild()
	_, err := tiqq.NewRow(stmt, int64(1))

	require.EqualError(t, err, "tiqq: got 1 values for 2 projected columns")
}

func TestBuildRequiresProjection(t *testing.T) {
	j := UserTable.LeftJoin(AddressTable).On(tiqq.Eq(UserTable.ID, AddressTable.UserID))
	_, err := j.Where(j.Left().ID.Eq(1)).Build()

	require.EqualError(t, err, "tiqq: SELECT requires at least one column")
}

func TestMustBuildPanicsOnValidationError(t *testing.T) {
	j := UserTable.LeftJoin(AddressTable).On(tiqq.Eq(UserTable.ID, AddressTable.UserID))

	require.PanicsWithError(t, "tiqq: SELECT requires at least one column", func() {
		j.Where(j.Left().ID.Eq(1)).MustBuild()
	})
}

func TestStatementArgsReturnsCopy(t *testing.T) {
	j := UserTable.LeftJoin(AddressTable).On(tiqq.Eq(UserTable.ID, AddressTable.UserID))
	stmt := j.Select(j.Left().ID).Where(j.Left().ID.Eq(1)).MustBuild()
	args := stmt.Args()
	args[0] = int64(999)

	require.Equal(t, []any{int64(1)}, stmt.Args())
}

func TestQueryIsImmutable(t *testing.T) {
	j := UserTable.LeftJoin(AddressTable).On(tiqq.Eq(UserTable.ID, AddressTable.UserID))
	base := j.Select(j.Left().ID)
	filtered := base.Where(j.Left().ID.Eq(1))

	require.NotContains(t, base.MustBuild().SQL(), " WHERE ")
	require.Contains(t, filtered.MustBuild().SQL(), ` WHERE "users"."id" = $1`)
}
