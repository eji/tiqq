package codegen_test

import (
	"testing"

	"github.com/eji/tiqq/codegen"
	"github.com/eji/tiqq/schema"
	"github.com/stretchr/testify/require"
)

func TestGenerate(t *testing.T) {
	database := schema.Schema{Tables: []schema.Table{
		{
			Name: "users",
			Columns: []schema.Column{
				{Name: "id", DBType: "int8"},
				{Name: "display_name", DBType: "text"},
				{Name: "balance", DBType: "numeric"},
			},
		},
		{
			Name: "addresses",
			Columns: []schema.Column{
				{Name: "id", DBType: "int8"},
				{Name: "user_id", DBType: "int8"},
				{Name: "address", DBType: "text", Nullable: true},
			},
			ForeignKeys: []schema.ForeignKey{{
				Name: "addresses_user_id_fkey", Columns: []string{"user_id"},
				ReferencedTable: "users", ReferencedColumns: []string{"id"},
			}},
		},
	}}

	generated, err := codegen.Generate(database, codegen.Config{Package: "dbschema"})
	source := string(generated)

	require.NoError(t, err)
	require.Contains(t, source, "type UserTableDef struct")
	require.Contains(t, source, "DisplayName tiqq.Column[UserScope, string, string]")
	require.Contains(t, source, "tiqq.Column[UserScope, tiqq.Decimal, tiqq.Decimal]")
	require.Contains(t, source, "Address tiqq.Column[AddressScope, sql.Null[string], string]")
	require.Contains(t, source, "func (table UserTableDef) InnerJoin(right AddressTableDef")
	require.Contains(t, source, "func (table UserTableDef) LeftJoin(right AddressTableDef")
	require.Contains(t, source, `tiqq.NewInnerJoinSource("users", "addresses", on)`)
	require.Contains(t, source, `tiqq.NewLeftJoinSource("users", "addresses", on)`)
}

func TestGenerateValidation(t *testing.T) {
	tests := map[string]struct {
		database schema.Schema
		config   codegen.Config
		want     string
	}{
		"package is required": {
			want: "codegen: package name is required",
		},
		"referenced table must exist": {
			config: codegen.Config{Package: "dbschema"},
			database: schema.Schema{Tables: []schema.Table{{
				Name:        "addresses",
				ForeignKeys: []schema.ForeignKey{{Name: "missing_fk", ReferencedTable: "users"}},
			}}},
			want: "codegen: foreign key missing_fk references unknown table users",
		},
		"join relation must be unambiguous": {
			config: codegen.Config{Package: "dbschema"},
			database: schema.Schema{Tables: []schema.Table{
				{Name: "users"},
				{Name: "addresses", ForeignKeys: []schema.ForeignKey{{ReferencedTable: "users"}}},
				{Name: "profiles", ForeignKeys: []schema.ForeignKey{{ReferencedTable: "users"}}},
			}},
			want: "codegen: table users has multiple join relations; explicit relation API is required",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := codegen.Generate(test.database, test.config)
			require.EqualError(t, err, test.want)
		})
	}
}
