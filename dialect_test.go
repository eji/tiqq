package tiqq

import (
	"testing"

	"github.com/stretchr/testify/require"
)

type testDialect struct{ renderer sqlRenderer }

func (dialect testDialect) dialectRenderer() sqlRenderer { return dialect.renderer }

type testRenderer struct{ dialectName string }

func (renderer testRenderer) name() string               { return renderer.dialectName }
func (testRenderer) quoteIdentifier(value string) string { return "[" + value + "]" }
func (testRenderer) placeholder(index int) string        { return "?" }

func TestDialectRenderer(t *testing.T) {
	tests := map[string]struct {
		dialect Dialect
		render  func(sqlRenderer) string
		want    string
	}{
		"postgres identifier": {
			dialect: PostgreSQL,
			render:  func(renderer sqlRenderer) string { return renderer.quoteIdentifier(`a"b`) },
			want:    `"a""b"`,
		},
		"postgres placeholder": {
			dialect: PostgreSQL,
			render:  func(renderer sqlRenderer) string { return renderer.placeholder(3) },
			want:    "$3",
		},
		"mysql identifier": {
			dialect: MySQL,
			render:  func(renderer sqlRenderer) string { return renderer.quoteIdentifier("a`b") },
			want:    "`a``b`",
		},
		"mysql placeholder": {
			dialect: MySQL,
			render:  func(renderer sqlRenderer) string { return renderer.placeholder(3) },
			want:    "?",
		},
		"sqlite identifier": {
			dialect: SQLite,
			render:  func(renderer sqlRenderer) string { return renderer.quoteIdentifier(`a"b`) },
			want:    `"a""b"`,
		},
		"sqlite placeholder": {
			dialect: SQLite,
			render:  func(renderer sqlRenderer) string { return renderer.placeholder(3) },
			want:    "?",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, test.want, test.render(test.dialect.dialectRenderer()))
		})
	}
}

type mysqlScope struct{}

func TestMySQLBuildRendering(t *testing.T) {
	schema := NewSchemaInfo(MySQL)
	table := schema.Table("users")
	id := RequiredNumericColumn[mysqlScope, int64, Decimal, Decimal]("users", "id")
	selectStatement := NewQuery(source{tables: []tableSource{{ref: table}}}).
		Where(id.Eq(int64(7))).
		Select(id).
		MustBuild()
	insertStatement := NewInsert[mysqlScope, MySQLMarker](
		table, []string{"id"}, []string{"id"}, [][]string{{"id"}},
	).Values(id.Value(int64(7))).MustBuild()
	updateStatement := UpdateQuery[mysqlScope]{table: tableSource{ref: table}}.
		Set(id.To(int64(8))).
		Where(id.Eq(int64(7))).
		MustBuild()

	require.Equal(t, "SELECT `users`.`id` FROM `users` WHERE `users`.`id` = ?", selectStatement.SQL())
	require.Equal(t, []any{int64(7)}, selectStatement.Args())
	require.Equal(t, "INSERT INTO `users` (`id`) VALUES (?)", insertStatement.SQL())
	require.Equal(t, []any{int64(7)}, insertStatement.Args())
	require.Equal(t, "UPDATE `users` SET `id` = ? WHERE `users`.`id` = ?", updateStatement.SQL())
	require.Equal(t, []any{int64(8), int64(7)}, updateStatement.Args())
}

func TestQueryRendererRejectsMixedDialects(t *testing.T) {
	postgres := NewSchemaInfo(PostgreSQL)
	other := NewSchemaInfo(testDialect{renderer: testRenderer{dialectName: "other"}})

	_, err := queryRenderer([]tableSource{
		{ref: postgres.Table("users")},
		{ref: other.Table("addresses")},
	})

	require.EqualError(t, err, "tiqq: cannot combine postgresql and other SQL dialects")
}

func TestSelectModifiersUseDialectPlaceholders(t *testing.T) {
	tests := map[string]struct {
		dialect Dialect
		want    string
	}{
		"mysql": {
			dialect: MySQL,
			want:    "SELECT DISTINCT `users`.`id` FROM `users` ORDER BY `users`.`id` DESC LIMIT ? OFFSET ?",
		},
		"sqlite": {
			dialect: SQLite,
			want:    `SELECT DISTINCT "users"."id" FROM "users" ORDER BY "users"."id" DESC LIMIT ? OFFSET ?`,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			table := NewSchemaInfo(test.dialect).Table("users")
			id := RequiredNumericColumn[mysqlScope, int64, Decimal, Decimal]("users", "id")
			statement := NewQuery(source{tables: []tableSource{{ref: table}}}).
				Select(id).Distinct().OrderBy(id.Desc()).Limit(10).Offset(5).MustBuild()

			require.Equal(t, test.want, statement.SQL())
			require.Equal(t, []any{int64(10), int64(5)}, statement.Args())
		})
	}
}

func TestOffsetWithoutLimitValidation(t *testing.T) {
	tests := map[string]struct {
		dialect Dialect
		want    string
	}{
		"mysql": {
			dialect: MySQL,
			want:    "tiqq: mysql OFFSET requires LIMIT",
		},
		"sqlite": {
			dialect: SQLite,
			want:    "tiqq: sqlite OFFSET requires LIMIT",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			table := NewSchemaInfo(test.dialect).Table("users")
			id := RequiredNumericColumn[mysqlScope, int64, Decimal, Decimal]("users", "id")
			_, err := NewQuery(source{tables: []tableSource{{ref: table}}}).Select(id).Offset(5).Build()

			require.EqualError(t, err, test.want)
		})
	}
}
