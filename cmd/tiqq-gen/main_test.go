package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRun(t *testing.T) {
	directory := t.TempDir()
	input := filepath.Join(directory, "schema.json")
	output := filepath.Join(directory, "schema_gen.go")
	require.NoError(t, os.WriteFile(input, []byte(`{
		"Name":"public",
		"Tables":[{"Name":"users","Columns":[{"Name":"id","DBType":"int8"}]}]
	}`), 0o600))

	err := run("dbschema", input, output)
	generated, readErr := os.ReadFile(output)

	require.NoError(t, err)
	require.NoError(t, readErr)
	require.Contains(t, string(generated), "package dbschema")
	require.Contains(t, string(generated), "ID tiqq.Column[UserScope, int64, int64]")
}
