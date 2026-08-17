package postgres

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

	mock.ExpectQuery(regexp.QuoteMeta(columnsSQL)).
		WithArgs("public").
		WillReturnRows(sqlmock.NewRows([]string{
			"table_name", "column_name", "udt_name", "is_nullable",
			"column_default", "is_identity", "is_generated",
		}).
			AddRow("addresses", "id", "int8", "NO", nil, "YES", "NEVER").
			AddRow("addresses", "user_id", "int8", "NO", nil, "NO", "NEVER").
			AddRow("addresses", "address", "text", "YES", "'unknown'::text", "NO", "ALWAYS").
			AddRow("users", "id", "int8", "NO", nil, "YES", "NEVER").
			AddRow("users", "name", "text", "NO", nil, "NO", "NEVER"))
	mock.ExpectQuery(regexp.QuoteMeta(keysSQL)).
		WithArgs("public").
		WillReturnRows(sqlmock.NewRows([]string{"table_name", "constraint_name", "constraint_type", "column_name"}).
			AddRow("addresses", "addresses_pkey", "PRIMARY KEY", "id").
			AddRow("users", "users_name_key", "UNIQUE", "name").
			AddRow("users", "users_pkey", "PRIMARY KEY", "id"))
	mock.ExpectQuery(regexp.QuoteMeta(foreignKeysSQL)).
		WithArgs("public").
		WillReturnRows(sqlmock.NewRows([]string{
			"table_name", "constraint_name", "column_name",
			"referenced_schema", "referenced_table", "referenced_column",
		}).AddRow("addresses", "addresses_user_id_fkey", "user_id", "public", "users", "id"))

	got, err := Introspect(context.Background(), database, "public")
	defaultAddress := "'unknown'::text"
	want := schema.Schema{Dialect: schema.PostgreSQL, Name: "public", Tables: []schema.Table{
		{
			Schema: "public", Name: "addresses",
			Columns: []schema.Column{
				{Name: "id", DBType: "int8", Identity: true},
				{Name: "user_id", DBType: "int8"},
				{Name: "address", DBType: "text", Nullable: true, Default: &defaultAddress, Generated: true},
			},
			PrimaryKey: &schema.Key{Name: "addresses_pkey", Columns: []string{"id"}},
			ForeignKeys: []schema.ForeignKey{{
				Name: "addresses_user_id_fkey", Columns: []string{"user_id"},
				ReferencedSchema: "public", ReferencedTable: "users", ReferencedColumns: []string{"id"},
			}},
		},
		{
			Schema: "public", Name: "users",
			Columns: []schema.Column{
				{Name: "id", DBType: "int8", Identity: true},
				{Name: "name", DBType: "text"},
			},
			PrimaryKey: &schema.Key{Name: "users_pkey", Columns: []string{"id"}},
			UniqueKeys: []schema.Key{{Name: "users_name_key", Columns: []string{"name"}}},
		},
	}}

	require.NoError(t, err)
	require.Equal(t, want, got)
	require.NoError(t, mock.ExpectationsWereMet())
}
