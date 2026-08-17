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
		"composite ambiguous relation requires explicit ON": {
			config: codegen.Config{Package: "dbschema"},
			database: schema.Schema{Tables: []schema.Table{
				{Name: "users"},
				{Name: "memberships", ForeignKeys: []schema.ForeignKey{{
					Name: "memberships_user_fkey", Columns: []string{"tenant_id", "user_id"},
					ReferencedTable: "users", ReferencedColumns: []string{"tenant_id", "id"},
				}}},
				{Name: "profiles", ForeignKeys: []schema.ForeignKey{{
					Name: "profiles_user_id_fkey", Columns: []string{"user_id"},
					ReferencedTable: "users", ReferencedColumns: []string{"id"},
				}}},
			}},
			want: "codegen: relation memberships_user_fkey uses a composite foreign key; explicit ON is required",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := codegen.Generate(test.database, test.config)
			require.EqualError(t, err, test.want)
		})
	}
}

func TestGenerateMultipleRelations(t *testing.T) {
	database := schema.Schema{Tables: []schema.Table{
		{Name: "users", Columns: []schema.Column{{Name: "id", DBType: "int8"}}},
		{
			Name:    "addresses",
			Columns: []schema.Column{{Name: "id", DBType: "int8"}, {Name: "user_id", DBType: "int8"}},
			ForeignKeys: []schema.ForeignKey{{
				Name: "addresses_user_id_fkey", Columns: []string{"user_id"},
				ReferencedTable: "users", ReferencedColumns: []string{"id"},
			}},
		},
		{
			Name:    "profiles",
			Columns: []schema.Column{{Name: "id", DBType: "int8"}, {Name: "user_id", DBType: "int8"}},
			ForeignKeys: []schema.ForeignKey{{
				Name: "profiles_user_id_fkey", Columns: []string{"user_id"},
				ReferencedTable: "users", ReferencedColumns: []string{"id"},
			}},
		},
	}}

	generated, err := codegen.Generate(database, codegen.Config{Package: "dbschema"})
	source := string(generated)

	require.NoError(t, err)
	require.Contains(t, source, "var AddressesUserID = AddressesUserIDRelationDef{}")
	require.Contains(t, source, "func (AddressesUserIDRelationDef) LeftJoin(left UserTableDef, right AddressTableDef)")
	require.Contains(t, source, "on := tiqq.On(left.ID, right.UserID)")
	require.Contains(t, source, "var ProfilesUserID = ProfilesUserIDRelationDef{}")
	require.Contains(t, source, "func (ProfilesUserIDRelationDef) InnerJoin(left UserTableDef, right ProfileTableDef)")
	require.NotContains(t, source, "func (table UserTableDef) LeftJoin(right AddressTableDef")
}

func TestGenerateSelfJoinAliases(t *testing.T) {
	database := schema.Schema{Tables: []schema.Table{{
		Name: "users",
		Columns: []schema.Column{
			{Name: "id", DBType: "int8"},
			{Name: "name", DBType: "text"},
			{Name: "manager_id", DBType: "int8", Nullable: true},
		},
		ForeignKeys: []schema.ForeignKey{{
			Name: "users_manager_id_fkey", Columns: []string{"manager_id"},
			ReferencedTable: "users", ReferencedColumns: []string{"id"},
		}},
	}}}

	generated, err := codegen.Generate(database, codegen.Config{Package: "dbschema"})
	source := string(generated)

	require.NoError(t, err)
	require.Contains(t, source, "type UserAliasTableDef struct")
	require.Contains(t, source, "func (table UserTableDef) As(alias string) UserAliasTableDef")
	require.Contains(t, source, "func (left UserAliasTableDef) LeftJoin(right UserAliasTableDef")
	require.Contains(t, source, "func (left UserAliasTableDef) InnerJoin(right UserAliasTableDef")
	require.Contains(t, source, `tiqq.NewAliasedLeftJoinSource("users", left.alias, "users", right.alias, on)`)
	require.NotContains(t, source, "func (table UserTableDef) LeftJoin(right UserTableDef")
}
