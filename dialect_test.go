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

func TestPostgresRenderer(t *testing.T) {
	tests := map[string]struct {
		render func(sqlRenderer) string
		want   string
	}{
		"identifier": {
			render: func(renderer sqlRenderer) string { return renderer.quoteIdentifier(`a"b`) },
			want:   `"a""b"`,
		},
		"placeholder": {
			render: func(renderer sqlRenderer) string { return renderer.placeholder(3) },
			want:   "$3",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, test.want, test.render(PostgreSQL.dialectRenderer()))
		})
	}
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
