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
	name := tiqq.RequiredColumn[userScope, string]("users", "name")
	query := mysql.NewInsert(tiqq.NewInsert[userScope, tiqq.MySQLMarker](
		table, []string{"id", "name"}, []string{"id", "name"}, [][]string{{"id"}},
	))
	tests := map[string]struct {
		query mysql.InsertQuery[userScope]
		sql   string
		args  []any
	}{
		"insert": {
			query: query.Values(id.Value(int64(7)), name.Value("Alice")),
			sql:   "INSERT INTO `users` (`id`, `name`) VALUES (?, ?)",
			args:  []any{int64(7), "Alice"},
		},
		"update from inserted row": {
			query: query.Values(id.Value(int64(7)), name.Value("Alice")).
				OnDuplicateKey().DoUpdate(mysql.Inserted(name)),
			sql: "INSERT INTO `users` (`id`, `name`) VALUES (?, ?) AS `tiqq_new` " +
				"ON DUPLICATE KEY UPDATE `name` = `tiqq_new`.`name`",
			args: []any{int64(7), "Alice"},
		},
		"update with explicit value": {
			query: query.Values(id.Value(int64(7)), name.Value("Alice")).
				OnDuplicateKey().DoUpdate(name.To("Existing")),
			sql: "INSERT INTO `users` (`id`, `name`) VALUES (?, ?) AS `tiqq_new` " +
				"ON DUPLICATE KEY UPDATE `name` = ?",
			args: []any{int64(7), "Alice", "Existing"},
		},
		"bulk insert": {
			query: query.Values(id.Value(int64(7)), name.Value("Alice")).
				Values(id.Value(int64(8)), name.Value("Bob")).
				OnDuplicateKey().DoUpdate(mysql.Inserted(name)),
			sql: "INSERT INTO `users` (`id`, `name`) VALUES (?, ?), (?, ?) AS `tiqq_new` " +
				"ON DUPLICATE KEY UPDATE `name` = `tiqq_new`.`name`",
			args: []any{int64(7), "Alice", int64(8), "Bob"},
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			statement := test.query.MustBuild()
			require.Equal(t, test.sql, statement.SQL())
			require.Equal(t, test.args, statement.Args())
		})
	}
}

func TestOnDuplicateKeyValidation(t *testing.T) {
	table := tiqq.NewSchemaInfo(tiqq.MySQL).Table("users")
	id := tiqq.RequiredNumericColumn[userScope, int64, tiqq.Decimal, tiqq.Decimal]("users", "id")
	name := tiqq.RequiredColumn[userScope, string]("users", "name")
	query := mysql.NewInsert(tiqq.NewInsert[userScope, tiqq.MySQLMarker](
		table, []string{"id", "name"}, []string{"id", "name"}, [][]string{{"id"}},
	))
	tests := map[string]struct {
		query mysql.InsertQuery[userScope]
		err   string
	}{
		"empty update": {
			query: query.Values(id.Value(int64(7)), name.Value("Alice")).OnDuplicateKey().DoUpdate(),
			err:   "tiqq: ON DUPLICATE KEY UPDATE requires at least one assignment",
		},
		"duplicate assignment": {
			query: query.Values(id.Value(int64(7)), name.Value("Alice")).
				OnDuplicateKey().DoUpdate(mysql.Inserted(name), name.To("Existing")),
			err: "tiqq: ON DUPLICATE KEY assignment column users.name is specified more than once",
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := test.query.Build()
			require.EqualError(t, err, test.err)
		})
	}
}
