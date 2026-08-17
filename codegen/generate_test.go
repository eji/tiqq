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
	require.Contains(t, source, "var tiqqSchema = tiqq.NewSchemaInfo(tiqq.PostgreSQL)")
	require.Contains(t, source, "type UserTableDef struct")
	require.Contains(t, source, "DisplayName tiqq.Column[UserScope, string, string]")
	require.Contains(t, source, "tiqq.NumericColumn[UserScope, tiqq.Decimal, tiqq.Decimal, tiqq.Decimal, tiqq.Decimal]")
	require.Contains(t, source, "Address tiqq.Column[AddressScope, sql.Null[string], string]")
	require.Contains(t, source, "type NullableAddressTableDef struct")
	require.Contains(t, source, "func (table UserTableDef) TiqqJoinSource() tiqq.JoinSourceInfo[UserTableDef]")
	require.Contains(t, source, "func (table UserTableDef) InnerJoin[C, NC any, R tiqq.TableLike[C, NC]]")
	require.Contains(t, source, "func (table UserTableDef) LeftJoin[C, NC any, R tiqq.TableLike[C, NC]]")
	require.Contains(t, source, "func (table UserTableDef) Update() tiqq.UpdateQuery[UserScope]")
	require.Contains(t, source, "func (table UserTableDef) Insert() postgres.InsertQuery[UserScope]")
	require.Contains(t, source, `tiqq.NewInsert[UserScope, tiqq.PostgreSQLMarker](table.ref, []string{"id", "display_name", "balance"}, []string{"id", "display_name", "balance"}, [][]string(nil))`)
	require.Contains(t, source, `Columns: []string{"user_id"}, ReferencedTable: "users", ReferencedColumns: []string{"id"}`)
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
		"dialect must be supported": {
			config:   codegen.Config{Package: "dbschema"},
			database: schema.Schema{Dialect: schema.Dialect("unknown")},
			want:     "codegen: unsupported SQL dialect unknown",
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
	require.Contains(t, source, "func (table UserTableDef) LeftJoin[C, NC any, R tiqq.TableLike[C, NC]]")
	require.Contains(t, source, "func (table AddressTableDef) LeftJoin[C, NC any, R tiqq.TableLike[C, NC]]")
	require.Contains(t, source, "func (table ProfileTableDef) InnerJoin[C, NC any, R tiqq.TableLike[C, NC]]")
	require.NotContains(t, source, "RelationDef")
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
	require.Contains(t, source, "func (table UserTableDef) As(alias string) UserTableDef")
	require.Contains(t, source, "func (table UserTableDef) LeftJoin[C, NC any, R tiqq.TableLike[C, NC]]")
	require.Contains(t, source, "table.ManagerID = tiqq.AliasNumericColumn[UserScope, UserScope](table.ManagerID, alias)")
	require.NotContains(t, source, "UserAliasTableDef")
}

func TestGenerateInsertMetadata(t *testing.T) {
	database := schema.Schema{Tables: []schema.Table{{
		Name: "users",
		Columns: []schema.Column{
			{Name: "id", DBType: "int8", Identity: true},
			{Name: "name", DBType: "text"},
			{Name: "nickname", DBType: "text", Nullable: true},
			{Name: "created_at", DBType: "timestamp", Default: stringPointer("now()")},
			{Name: "search_text", DBType: "text", Generated: true},
		},
	}}}

	generated, err := codegen.Generate(database, codegen.Config{Package: "dbschema"})
	source := string(generated)

	require.NoError(t, err)
	require.Contains(t, source, `tiqq.NewInsert[UserScope, tiqq.PostgreSQLMarker](table.ref, []string{"name", "nickname", "created_at"}, []string{"name"}, [][]string(nil))`)
}

func stringPointer(value string) *string { return &value }

func TestGeneratePostgresAggregateTypes(t *testing.T) {
	tests := map[string]struct {
		databaseType string
		want         string
	}{
		"integer": {
			databaseType: "int4",
			want:         "tiqq.NumericColumn[MetricScope, int32, int32, int64, tiqq.Decimal]",
		},
		"bigint": {
			databaseType: "int8",
			want:         "tiqq.NumericColumn[MetricScope, int64, int64, tiqq.Decimal, tiqq.Decimal]",
		},
		"numeric": {
			databaseType: "numeric",
			want:         "tiqq.NumericColumn[MetricScope, tiqq.Decimal, tiqq.Decimal, tiqq.Decimal, tiqq.Decimal]",
		},
		"real": {
			databaseType: "float4",
			want:         "tiqq.NumericColumn[MetricScope, float32, float32, float32, float64]",
		},
		"double precision": {
			databaseType: "float8",
			want:         "tiqq.NumericColumn[MetricScope, float64, float64, float64, float64]",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			generated, err := codegen.Generate(schema.Schema{Tables: []schema.Table{{
				Name: "metrics", Columns: []schema.Column{{Name: "metric", DBType: test.databaseType}},
			}}}, codegen.Config{Package: "dbschema"})

			require.NoError(t, err)
			require.Contains(t, string(generated), test.want)
		})
	}
}

func TestGeneratePostgresConflictKeys(t *testing.T) {
	database := schema.Schema{Dialect: schema.PostgreSQL, Tables: []schema.Table{{
		Name: "users",
		Columns: []schema.Column{
			{Name: "id", DBType: "int8"},
			{Name: "email", DBType: "text"},
		},
		PrimaryKey: &schema.Key{Name: "users_pkey", Columns: []string{"id"}},
		UniqueKeys: []schema.Key{{Name: "users_email_key", Columns: []string{"email"}}},
	}}}

	generated, err := codegen.Generate(database, codegen.Config{Package: "dbschema"})

	require.NoError(t, err)
	require.Contains(t, string(generated), `[][]string{[]string{"id"}, []string{"email"}}`)
}
