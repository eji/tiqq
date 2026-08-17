package main

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/eji/tiqq/codegen"
	"github.com/eji/tiqq/schema"
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
	require.Contains(t, string(generated), "tiqq.Column[UserScope, int64, int64]")
}

func TestRunPostgresCLI(t *testing.T) {
	database, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	require.NoError(t, err)
	t.Cleanup(func() { _ = database.Close() })
	mock.ExpectPing()
	var output bytes.Buffer
	wantSchema := schema.Schema{Name: "app"}
	dependencies := postgresDependencies{
		lookupEnv: func(name string) (string, bool) {
			require.Equal(t, "TIQQ_DATABASE_URL", name)
			return "postgres://secret@example/app", true
		},
		open: func(dsn string) (*sql.DB, error) {
			require.Equal(t, "postgres://secret@example/app", dsn)
			return database, nil
		},
		introspect: func(ctx context.Context, gotDatabase *sql.DB, schemaName string) (schema.Schema, error) {
			require.Same(t, database, gotDatabase)
			require.Equal(t, "app", schemaName)
			return wantSchema, nil
		},
		generate: func(gotSchema schema.Schema, config codegen.Config) ([]byte, error) {
			require.Equal(t, wantSchema, gotSchema)
			require.Equal(t, "dbschema", config.Package)
			return []byte("generated"), nil
		},
		write:  writeGenerated,
		stdout: &output,
	}

	err = runPostgresCLI([]string{
		"-dsn-env", "TIQQ_DATABASE_URL",
		"-schema", "app",
		"-package", "dbschema",
		"-output", "-",
	}, dependencies)

	require.NoError(t, err)
	require.Equal(t, "generated", output.String())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRunPostgresCLIValidation(t *testing.T) {
	tests := map[string]struct {
		arguments    []string
		dependencies postgresDependencies
		want         string
	}{
		"DSN environment is required": {
			dependencies: postgresDependencies{
				lookupEnv: func(name string) (string, bool) { return "", false },
			},
			want: "PostgreSQL DSN environment variable DATABASE_URL is not set",
		},
		"invalid duration is rejected": {
			arguments: []string{"-timeout", "invalid"},
			dependencies: postgresDependencies{
				lookupEnv: func(name string) (string, bool) { return "unused", true },
			},
			want: "invalid value \"invalid\" for flag -timeout",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			err := runPostgresCLI(test.arguments, test.dependencies)
			require.ErrorContains(t, err, test.want)
		})
	}
}

func TestRunPostgresCLIDoesNotExposeDSNOnOpenError(t *testing.T) {
	secret := "postgres://user:super-secret@example/app"
	dependencies := postgresDependencies{
		lookupEnv: func(name string) (string, bool) { return secret, true },
		open: func(dsn string) (*sql.DB, error) {
			return nil, errors.New("invalid PostgreSQL DSN")
		},
	}

	err := runPostgresCLI(nil, dependencies)

	require.EqualError(t, err, "invalid PostgreSQL DSN")
	require.NotContains(t, err.Error(), secret)
}

func TestDefaultPostgresOpenDoesNotExposeInvalidDSN(t *testing.T) {
	secret := "postgres://user:super-secret@%"
	_, err := defaultPostgresDependencies().open(secret)

	require.EqualError(t, err, "invalid PostgreSQL DSN")
	require.NotContains(t, err.Error(), secret)
}

func TestWriteGeneratedAtomicallyReplacesOutput(t *testing.T) {
	output := filepath.Join(t.TempDir(), "schema_gen.go")
	require.NoError(t, os.WriteFile(output, []byte("old"), 0o600))

	err := writeGenerated(output, []byte("new"), io.Discard)
	generated, readErr := os.ReadFile(output)
	info, statErr := os.Stat(output)

	require.NoError(t, err)
	require.NoError(t, readErr)
	require.NoError(t, statErr)
	require.Equal(t, "new", string(generated))
	require.Equal(t, os.FileMode(0o644), info.Mode().Perm())
}
