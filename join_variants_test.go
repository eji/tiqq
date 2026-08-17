package tiqq_test

import (
	"database/sql"
	"testing"

	"github.com/eji/tiqq"
	. "github.com/eji/tiqq/example/schema"
	"github.com/stretchr/testify/require"
)

func TestJoinVariantsBuild(t *testing.T) {
	tests := map[string]struct {
		build   func() tiqq.Statement
		wantSQL string
	}{
		"right join": {
			build: func() tiqq.Statement {
				joined := UserTable.RightJoin(AddressTable)
				return joined.Select(joined.Left().ID, joined.Right().Address).MustBuild()
			},
			wantSQL: `SELECT "users"."id", "addresses"."address" FROM "users" RIGHT JOIN "addresses" ON "addresses"."user_id" = "users"."id"`,
		},
		"full join": {
			build: func() tiqq.Statement {
				joined := UserTable.FullJoin(AddressTable)
				return joined.Select(joined.Left().ID, joined.Right().Address).MustBuild()
			},
			wantSQL: `SELECT "users"."id", "addresses"."address" FROM "users" FULL JOIN "addresses" ON "addresses"."user_id" = "users"."id"`,
		},
		"cross join": {
			build: func() tiqq.Statement {
				joined := UserTable.CrossJoin(CompanyTable)
				return joined.Select(joined.Left().ID, joined.Right().Name).MustBuild()
			},
			wantSQL: `SELECT "users"."id", "companies"."name" FROM "users" CROSS JOIN "companies"`,
		},
		"nested right join nullifies the complete left tree": {
			build: func() tiqq.Statement {
				userAddress := UserTable.LeftJoin(AddressTable)
				joined := tiqq.RightJoin(userAddress, CompanyTable)
				return joined.Select(
					joined.Left().Left().ID,
					joined.Left().Right().Address,
					joined.Right().Name,
				).MustBuild()
			},
			wantSQL: `SELECT "users"."id", "addresses"."address", "companies"."name" FROM "users" LEFT JOIN "addresses" ON "addresses"."user_id" = "users"."id" RIGHT JOIN "companies" ON "addresses"."company_id" = "companies"."id"`,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, test.wantSQL, test.build().SQL())
		})
	}
}

func TestOuterJoinResultNullability(t *testing.T) {
	right := UserTable.RightJoin(AddressTable)
	rightStatement := right.Select(right.Left().ID, right.Right().Address).MustBuild()
	rightRow, err := tiqq.NewRow(rightStatement, nil, "Tokyo")
	rightUserID, rightUserIDErr := rightRow.Get(right.Left().ID)
	rightAddress, rightAddressErr := rightRow.Get(right.Right().Address)

	full := UserTable.FullJoin(AddressTable)
	fullStatement := full.Select(full.Left().ID, full.Right().Address).MustBuild()
	fullRow, fullRowErr := tiqq.NewRow(fullStatement, nil, nil)
	fullUserID, fullUserIDErr := fullRow.Get(full.Left().ID)
	fullAddress, fullAddressErr := fullRow.Get(full.Right().Address)

	require.NoError(t, err)
	require.NoError(t, rightUserIDErr)
	require.NoError(t, rightAddressErr)
	require.Equal(t, sql.Null[int64]{}, rightUserID)
	require.Equal(t, "Tokyo", rightAddress)
	require.NoError(t, fullRowErr)
	require.NoError(t, fullUserIDErr)
	require.NoError(t, fullAddressErr)
	require.Equal(t, sql.Null[int64]{}, fullUserID)
	require.Equal(t, sql.Null[string]{}, fullAddress)
}

func TestCrossJoinRejectsOn(t *testing.T) {
	joined := UserTable.CrossJoin(CompanyTable).On(tiqq.Eq(UserTable.ID, CompanyTable.ID))

	_, err := joined.Select(joined.Left().ID).Build()

	require.EqualError(t, err, "tiqq: CROSS JOIN does not accept ON")
}
