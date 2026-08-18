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
	require.Contains(t, source, "func (table UserTableDef) TiqqJoinSource() tiqq.JoinSourceInfo[UserTableDef, NullableUserTableDef]")
	require.Contains(t, source, "func (table UserTableDef) InnerJoin[C, NC any, R tiqq.TableLike[C, NC]]")
	require.Contains(t, source, "func (table UserTableDef) LeftJoin[C, NC any, R tiqq.TableLike[C, NC]]")
	require.Contains(t, source, "func (table UserTableDef) RightJoin[C, NC any, R tiqq.TableLike[C, NC]]")
	require.Contains(t, source, "func (table UserTableDef) FullJoin[C, NC any, R tiqq.TableLike[C, NC]]")
	require.Contains(t, source, "func (table UserTableDef) CrossJoin[C, NC any, R tiqq.TableLike[C, NC]]")
	require.Contains(t, source, "func (table UserTableDef) Update() tiqq.UpdateQuery[UserScope]")
	require.Contains(t, source, "func (table UserTableDef) Delete() tiqq.DeleteQuery[UserScope]")
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

func TestGenerateMySQL(t *testing.T) {
	database := schema.Schema{Dialect: schema.MySQL, Tables: []schema.Table{{
		Name: "metrics",
		Columns: []schema.Column{
			{Name: "id", DBType: "bigint", Unsigned: true},
			{Name: "amount", DBType: "decimal"},
			{Name: "ratio", DBType: "float"},
			{Name: "payload", DBType: "blob", Nullable: true},
		},
		PrimaryKey: &schema.Key{Name: "metrics_pkey", Columns: []string{"id"}},
	}}}

	generated, err := codegen.Generate(database, codegen.Config{Package: "dbschema"})
	source := string(generated)

	require.NoError(t, err)
	require.Contains(t, source, `"github.com/eji/tiqq/mysql"`)
	require.NotContains(t, source, `"github.com/eji/tiqq/postgres"`)
	require.Contains(t, source, "var tiqqSchema = tiqq.NewSchemaInfo(tiqq.MySQL)")
	require.Contains(t, source, "tiqq.NumericColumn[MetricScope, uint64, uint64, tiqq.Decimal, tiqq.Decimal]")
	require.Contains(t, source, "tiqq.NumericColumn[MetricScope, float32, float32, float64, float64]")
	require.Contains(t, source, "tiqq.Column[MetricScope, sql.Null[[]byte], []byte]")
	require.Contains(t, source, "func (table MetricTableDef) Insert() mysql.InsertQuery[MetricScope]")
	require.Contains(t, source, "tiqq.NewInsert[MetricScope, tiqq.MySQLMarker]")
}

func TestGenerateSQLite(t *testing.T) {
	database := schema.Schema{Dialect: schema.SQLite, Tables: []schema.Table{{
		Name: "metrics",
		Columns: []schema.Column{
			{Name: "id", DBType: "integer", Identity: true},
			{Name: "ratio", DBType: "double"},
			{Name: "payload", DBType: "blob", Nullable: true},
			{Name: "label", DBType: "varchar(255)"},
			{Name: "amount", DBType: "decimal(20, 4)"},
		},
	}}}

	generated, err := codegen.Generate(database, codegen.Config{Package: "dbschema"})
	source := string(generated)

	require.NoError(t, err)
	require.Contains(t, source, `"github.com/eji/tiqq/sqlite"`)
	require.Contains(t, source, "var tiqqSchema = tiqq.NewSchemaInfo(tiqq.SQLite)")
	require.Contains(t, source, "tiqq.NumericColumn[MetricScope, int64, int64, int64, float64]")
	require.Contains(t, source, "tiqq.NumericColumn[MetricScope, float64, float64, float64, float64]")
	require.Contains(t, source, "tiqq.Column[MetricScope, sql.Null[[]byte], []byte]")
	require.Contains(t, source, "tiqq.Column[MetricScope, string, string]")
	require.Contains(t, source, "tiqq.NumericColumn[MetricScope, tiqq.Decimal, tiqq.Decimal, tiqq.Decimal, tiqq.Decimal]")
	require.Contains(t, source, "func (table MetricTableDef) Insert() sqlite.InsertQuery[MetricScope]")
	require.Contains(t, source, "tiqq.NewInsert[MetricScope, tiqq.SQLiteMarker]")
}

func TestGenerateStandardUUIDAndJSONTypes(t *testing.T) {
	tests := map[string]struct {
		dialect      schema.Dialect
		columns      []schema.Column
		wantTypes    []string
		unwantedType string
	}{
		"postgres": {
			dialect: schema.PostgreSQL,
			columns: []schema.Column{
				{Name: "id", DBType: "uuid"},
				{Name: "payload", DBType: "jsonb", Nullable: true},
			},
			wantTypes: []string{
				`"encoding/json/jsontext"`, `"uuid"`,
				"tiqq.Column[EventScope, uuid.UUID, uuid.UUID]",
				"tiqq.Column[EventScope, sql.Null[jsontext.Value], jsontext.Value]",
			},
			unwantedType: `"github.com/eji/tiqq/mysql"`,
		},
		"mysql": {
			dialect: schema.MySQL,
			columns: []schema.Column{
				{Name: "id", DBType: "varchar"},
				{Name: "payload", DBType: "json"},
			},
			wantTypes: []string{
				`"encoding/json/jsontext"`,
				"tiqq.Column[EventScope, jsontext.Value, jsontext.Value]",
			},
			unwantedType: `"uuid"`,
		},
		"sqlite declared types": {
			dialect: schema.SQLite,
			columns: []schema.Column{
				{Name: "id", DBType: "uuid"},
				{Name: "payload", DBType: "json"},
			},
			wantTypes: []string{
				`"encoding/json/jsontext"`, `"uuid"`,
				"tiqq.Column[EventScope, uuid.UUID, uuid.UUID]",
				"tiqq.Column[EventScope, jsontext.Value, jsontext.Value]",
			},
			unwantedType: `"github.com/eji/tiqq/postgres"`,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			generated, err := codegen.Generate(schema.Schema{
				Dialect: test.dialect,
				Tables:  []schema.Table{{Name: "events", Columns: test.columns}},
			}, codegen.Config{Package: "dbschema"})
			source := string(generated)

			require.NoError(t, err)
			for _, want := range test.wantTypes {
				require.Contains(t, source, want)
			}
			require.NotContains(t, source, test.unwantedType)
		})
	}
}

func TestGenerateStandardTimeTypes(t *testing.T) {
	tests := map[string]struct {
		dialect   schema.Dialect
		columns   []schema.Column
		wantTypes []string
	}{
		"postgres": {
			dialect: schema.PostgreSQL,
			columns: []schema.Column{
				{Name: "created_at", DBType: "timestamptz"},
				{Name: "published_on", DBType: "date", Nullable: true},
				{Name: "local_time", DBType: "time"},
			},
			wantTypes: []string{
				`"time"`,
				"tiqq.Column[EventScope, time.Time, time.Time]",
				"tiqq.Column[EventScope, sql.Null[time.Time], time.Time]",
				"tiqq.Column[EventScope, string, string]",
			},
		},
		"mysql": {
			dialect: schema.MySQL,
			columns: []schema.Column{
				{Name: "created_at", DBType: "datetime"},
				{Name: "published_on", DBType: "date", Nullable: true},
				{Name: "local_time", DBType: "time"},
			},
			wantTypes: []string{
				`"time"`,
				"tiqq.Column[EventScope, time.Time, time.Time]",
				"tiqq.Column[EventScope, sql.Null[time.Time], time.Time]",
				"tiqq.Column[EventScope, string, string]",
			},
		},
		"sqlite declared types": {
			dialect: schema.SQLite,
			columns: []schema.Column{
				{Name: "created_at", DBType: "datetime"},
				{Name: "published_on", DBType: "date", Nullable: true},
				{Name: "local_time", DBType: "time"},
			},
			wantTypes: []string{
				`"time"`,
				"tiqq.Column[EventScope, time.Time, time.Time]",
				"tiqq.Column[EventScope, sql.Null[time.Time], time.Time]",
				"tiqq.NumericColumn[EventScope, tiqq.Decimal, tiqq.Decimal, tiqq.Decimal, tiqq.Decimal]",
			},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			generated, err := codegen.Generate(schema.Schema{
				Dialect: test.dialect,
				Tables:  []schema.Table{{Name: "events", Columns: test.columns}},
			}, codegen.Config{Package: "dbschema"})
			source := string(generated)

			require.NoError(t, err)
			for _, want := range test.wantTypes {
				require.Contains(t, source, want)
			}
		})
	}
}
