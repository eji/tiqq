package mysql

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
		WithArgs("app").
		WillReturnRows(sqlmock.NewRows([]string{
			"table_name", "column_name", "data_type", "is_nullable", "column_default",
			"extra", "generation_expression", "column_type",
		}).
			AddRow("addresses", "id", "bigint", "NO", nil, "auto_increment", "", "bigint unsigned").
			AddRow("addresses", "user_id", "bigint", "NO", nil, "", "", "bigint unsigned").
			AddRow("addresses", "address", "varchar", "YES", "unknown", "", "", "varchar(255)").
			AddRow("addresses", "normalized", "varchar", "YES", nil, "STORED GENERATED", "lower(`address`)", "varchar(255)").
			AddRow("users", "id", "bigint", "NO", nil, "auto_increment", "", "bigint unsigned").
			AddRow("users", "email", "varchar", "NO", nil, "", "", "varchar(255)"))
	mock.ExpectQuery(regexp.QuoteMeta(keysSQL)).
		WithArgs("app").
		WillReturnRows(sqlmock.NewRows([]string{"table_name", "constraint_name", "constraint_type", "column_name"}).
			AddRow("addresses", "PRIMARY", "PRIMARY KEY", "id").
			AddRow("users", "PRIMARY", "PRIMARY KEY", "id").
			AddRow("users", "users_email_key", "UNIQUE", "email"))
	mock.ExpectQuery(regexp.QuoteMeta(foreignKeysSQL)).
		WithArgs("app").
		WillReturnRows(sqlmock.NewRows([]string{
			"table_name", "constraint_name", "column_name",
			"referenced_table_schema", "referenced_table_name", "referenced_column_name",
		}).AddRow("addresses", "addresses_user_id_fkey", "user_id", "app", "users", "id"))

	got, err := Introspect(context.Background(), database, "app")
	defaultAddress := "unknown"
	want := schema.Schema{Dialect: schema.MySQL, Name: "app", Tables: []schema.Table{
		{
			Schema: "app", Name: "addresses",
			Columns: []schema.Column{
				{Name: "id", DBType: "bigint", Identity: true, Unsigned: true},
				{Name: "user_id", DBType: "bigint", Unsigned: true},
				{Name: "address", DBType: "varchar", Nullable: true, Default: &defaultAddress},
				{Name: "normalized", DBType: "varchar", Nullable: true, Generated: true},
			},
			PrimaryKey: &schema.Key{Name: "PRIMARY", Columns: []string{"id"}},
			ForeignKeys: []schema.ForeignKey{{
				Name: "addresses_user_id_fkey", Columns: []string{"user_id"},
				ReferencedSchema: "app", ReferencedTable: "users", ReferencedColumns: []string{"id"},
			}},
		},
		{
			Schema: "app", Name: "users",
			Columns: []schema.Column{
				{Name: "id", DBType: "bigint", Identity: true, Unsigned: true},
				{Name: "email", DBType: "varchar"},
			},
			PrimaryKey: &schema.Key{Name: "PRIMARY", Columns: []string{"id"}},
			UniqueKeys: []schema.Key{{Name: "users_email_key", Columns: []string{"email"}}},
		},
	}}

	require.NoError(t, err)
	require.Equal(t, want, got)
	require.NoError(t, mock.ExpectationsWereMet())
}
