package mysql_test

import (
	"testing"

	"github.com/eji/tiqq"
	"github.com/eji/tiqq/mysql"
	"github.com/stretchr/testify/require"
)

type userScope struct{}

func TestInsertBuild(t *testing.T) {
	table := tiqq.NewSchemaInfo(tiqq.MySQL).Table("users")
	id := tiqq.RequiredNumericColumn[userScope, int64, tiqq.Decimal, tiqq.Decimal]("users", "id")
	query := mysql.NewInsert(tiqq.NewInsert[userScope, tiqq.MySQLMarker](
		table, []string{"id"}, []string{"id"}, [][]string{{"id"}},
	))
	statement := query.Values(id.Value(int64(7))).MustBuild()

	require.Equal(t, "INSERT INTO `users` (`id`) VALUES (?)", statement.SQL())
	require.Equal(t, []any{int64(7)}, statement.Args())
}
