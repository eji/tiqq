package sqlite

import (
	"context"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/eji/tiqq/schema"
	"github.com/stretchr/testify/require"
)

func TestIntrospect(t *testing.T) {
	database, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = database.Close() })

	mock.ExpectQuery(regexp.QuoteMeta(tablesSQL)).WillReturnRows(
		sqlmock.NewRows([]string{"name"}).AddRow("addresses").AddRow("users"),
	)
	mock.ExpectQuery(regexp.QuoteMeta(`PRAGMA table_xinfo("addresses")`)).WillReturnRows(
		sqlmock.NewRows([]string{"cid", "name", "type", "notnull", "dflt_value", "pk", "hidden"}).
			AddRow(0, "id", "INTEGER", 0, nil, 1, 0).
			AddRow(1, "user_id", "INTEGER", 1, nil, 0, 0).
			AddRow(2, "address", "TEXT", 0, "'unknown'", 0, 0).
			AddRow(3, "normalized", "TEXT", 0, nil, 0, 3),
	)
	mock.ExpectQuery(regexp.QuoteMeta(`PRAGMA index_list("addresses")`)).WillReturnRows(
		sqlmock.NewRows([]string{"seq", "name", "unique", "origin", "partial"}),
	)
	mock.ExpectQuery(regexp.QuoteMeta(`PRAGMA foreign_key_list("addresses")`)).WillReturnRows(
		sqlmock.NewRows([]string{"id", "seq", "table", "from", "to", "on_update", "on_delete", "match"}).
			AddRow(0, 0, "users", "user_id", nil, "NO ACTION", "NO ACTION", "NONE"),
	)
	mock.ExpectQuery(regexp.QuoteMeta(`PRAGMA table_xinfo("users")`)).WillReturnRows(
		sqlmock.NewRows([]string{"cid", "name", "type", "notnull", "dflt_value", "pk", "hidden"}).
			AddRow(0, "id", "INTEGER", 0, nil, 1, 0).
			AddRow(1, "email", "TEXT", 1, nil, 0, 0),
	)
	mock.ExpectQuery(regexp.QuoteMeta(`PRAGMA index_list("users")`)).WillReturnRows(
		sqlmock.NewRows([]string{"seq", "name", "unique", "origin", "partial"}).
			AddRow(0, "users_email_key", 1, "u", 0),
	)
	mock.ExpectQuery(regexp.QuoteMeta(`PRAGMA index_info("users_email_key")`)).WillReturnRows(
		sqlmock.NewRows([]string{"seqno", "cid", "name"}).AddRow(0, 1, "email"),
	)
	mock.ExpectQuery(regexp.QuoteMeta(`PRAGMA foreign_key_list("users")`)).WillReturnRows(
		sqlmock.NewRows([]string{"id", "seq", "table", "from", "to", "on_update", "on_delete", "match"}),
	)

	got, err := Introspect(context.Background(), database)
	defaultAddress := "'unknown'"
	want := schema.Schema{Dialect: schema.SQLite, Name: "main", Tables: []schema.Table{
		{
			Schema: "main", Name: "addresses",
			Columns: []schema.Column{
				{Name: "id", DBType: "integer", Identity: true},
				{Name: "user_id", DBType: "integer"},
				{Name: "address", DBType: "text", Nullable: true, Default: &defaultAddress},
				{Name: "normalized", DBType: "text", Nullable: true, Generated: true},
			},
			PrimaryKey: &schema.Key{Name: "addresses_pkey", Columns: []string{"id"}},
			ForeignKeys: []schema.ForeignKey{{
				Name: "addresses_fk_0", Columns: []string{"user_id"}, ReferencedSchema: "main",
				ReferencedTable: "users", ReferencedColumns: []string{"id"},
			}},
		},
		{
			Schema: "main", Name: "users",
			Columns: []schema.Column{
				{Name: "id", DBType: "integer", Identity: true},
				{Name: "email", DBType: "text"},
			},
			PrimaryKey: &schema.Key{Name: "users_pkey", Columns: []string{"id"}},
			UniqueKeys: []schema.Key{{Name: "users_email_key", Columns: []string{"email"}}},
		},
	}}

	require.NoError(t, err)
	require.Equal(t, want, got)
	require.NoError(t, mock.ExpectationsWereMet())
}
