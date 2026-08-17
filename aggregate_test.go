package tiqq_test

import (
	"database/sql"
	"testing"

	"github.com/eji/tiqq"
	. "github.com/eji/tiqq/example/schema"
	"github.com/stretchr/testify/require"
)

func TestGroupByHavingAndTypedAggregateResults(t *testing.T) {
	count := AddressTable.ID.Count()
	statement := AddressTable.
		Where(AddressTable.Address.Like("Tokyo%")).
		GroupBy(AddressTable.UserID).
		Having(count.Gt(int64(1))).
		Select(AddressTable.UserID, count).
		MustBuild()
	row, err := tiqq.NewRow(statement, int64(7), int64(3))
	userID, userIDErr := row.Get(AddressTable.UserID)
	countValue, countErr := row.Get(count)

	require.NoError(t, err)
	require.NoError(t, userIDErr)
	require.NoError(t, countErr)
	require.Equal(t, `SELECT "addresses"."user_id", COUNT("addresses"."id") FROM "addresses" WHERE "addresses"."address" LIKE $1 GROUP BY "addresses"."user_id" HAVING COUNT("addresses"."id") > $2`, statement.SQL())
	require.Equal(t, []any{"Tokyo%", int64(1)}, statement.Args())
	require.Equal(t, int64(7), userID)
	require.Equal(t, int64(3), countValue)
}

func TestPostgresAggregateResultTypes(t *testing.T) {
	sum := UserTable.ID.Sum()
	avg := UserTable.ID.Avg()
	minimum := UserTable.ID.Min()
	maximum := UserTable.Name.Max()
	statement := UserTable.Select(sum, avg, minimum, maximum).MustBuild()
	row, err := tiqq.NewRow(statement, "12", []byte("6"), int64(1), "Zoe")
	sumValue, sumErr := row.Get(sum)
	avgValue, avgErr := row.Get(avg)
	minimumValue, minimumErr := row.Get(minimum)
	maximumValue, maximumErr := row.Get(maximum)

	require.NoError(t, err)
	require.NoError(t, sumErr)
	require.NoError(t, avgErr)
	require.NoError(t, minimumErr)
	require.NoError(t, maximumErr)
	require.Equal(t, sql.Null[tiqq.Decimal]{V: "12", Valid: true}, sumValue)
	require.Equal(t, sql.Null[tiqq.Decimal]{V: "6", Valid: true}, avgValue)
	require.Equal(t, sql.Null[int64]{V: 1, Valid: true}, minimumValue)
	require.Equal(t, sql.Null[string]{V: "Zoe", Valid: true}, maximumValue)
}

func TestGroupingValidation(t *testing.T) {
	tests := map[string]struct {
		build func() error
		want  string
	}{
		"selected column must be grouped": {
			build: func() error {
				_, err := AddressTable.GroupBy(AddressTable.UserID).
					Select(AddressTable.UserID, AddressTable.Address, AddressTable.ID.Count()).
					Build()
				return err
			},
			want: "tiqq: SELECT column addresses.address must appear in GROUP BY",
		},
		"having column must be grouped": {
			build: func() error {
				_, err := AddressTable.GroupBy(AddressTable.UserID).
					Having(AddressTable.Address.Eq("Tokyo")).
					Select(AddressTable.UserID, AddressTable.ID.Count()).
					Build()
				return err
			},
			want: "tiqq: HAVING column addresses.address must appear in GROUP BY",
		},
		"aggregate cannot be grouped": {
			build: func() error {
				count := AddressTable.ID.Count()
				_, err := AddressTable.GroupBy(count).Select(count).Build()
				return err
			},
			want: `tiqq: GROUP BY does not accept aggregate COUNT(addresses.id)`,
		},
		"aggregate must belong to query": {
			build: func() error {
				_, err := UserTable.Select(AddressTable.ID.Count()).Build()
				return err
			},
			want: `tiqq: SELECT column COUNT(addresses.id) is not in query scope`,
		},
		"where does not accept aggregate": {
			build: func() error {
				count := AddressTable.ID.Count()
				_, err := AddressTable.Where(count.Gt(int64(1))).Select(count).Build()
				return err
			},
			want: `tiqq: WHERE does not accept aggregate COUNT(addresses.id)`,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			require.EqualError(t, test.build(), test.want)
		})
	}
}
