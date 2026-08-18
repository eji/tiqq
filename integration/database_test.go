//go:build integration

package integration_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/eji/tiqq"
	"github.com/eji/tiqq/codegen"
	mysqlintrospect "github.com/eji/tiqq/introspect/mysql"
	postgresintrospect "github.com/eji/tiqq/introspect/postgres"
	"github.com/eji/tiqq/schema"
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	mysqlcontainer "github.com/testcontainers/testcontainers-go/modules/mysql"
	postgrescontainer "github.com/testcontainers/testcontainers-go/modules/postgres"
)

type probeScope struct{}

type probeColumns struct {
	ID   tiqq.Column[probeScope, int64, int64]
	Name tiqq.Column[probeScope, string, string]
}

type probeTable struct {
	ref     tiqq.TableRef
	columns probeColumns
}

func (table probeTable) TiqqTableInfo() tiqq.TableInfo[probeColumns, probeColumns] {
	return tiqq.NewTableInfo(table.ref, table.columns, table.columns, nil)
}

func newProbeTable(dialect tiqq.Dialect) probeTable {
	return probeTable{
		ref: tiqq.NewSchemaInfo(dialect).Table("query_probes"),
		columns: probeColumns{
			ID:   tiqq.RequiredColumn[probeScope, int64]("query_probes", "id"),
			Name: tiqq.RequiredColumn[probeScope, string]("query_probes", "name"),
		},
	}
}

func assertProbeRoundTrip(t *testing.T, database *sql.DB, insert tiqq.Statement, table probeTable) {
	t.Helper()
	_, err := database.Exec(insert.SQL(), insert.Args()...)
	require.NoError(t, err)
	statement := tiqq.NewTableQuery(table).
		Where(table.columns.ID.Eq(int64(7))).
		Select(table.columns.ID, table.columns.Name).
		MustBuild()
	row, err := tiqq.ScanRow(database.QueryRow(statement.SQL(), statement.Args()...), statement)
	id, idErr := row.Get(table.columns.ID)
	name, nameErr := row.Get(table.columns.Name)
	require.NoError(t, err)
	require.NoError(t, idErr)
	require.NoError(t, nameErr)
	require.Equal(t, int64(7), id)
	require.Equal(t, "Alice", name)
}

func TestPostgreSQLIntrospectionAndCodeGeneration(t *testing.T) {
	ctx := context.Background()
	container, err := postgrescontainer.Run(
		ctx,
		"postgres:17-alpine",
		postgrescontainer.WithDatabase("tiqq"),
		postgrescontainer.WithUsername("tiqq"),
		postgrescontainer.WithPassword("tiqq"),
		postgrescontainer.BasicWaitStrategies(),
	)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, testcontainers.TerminateContainer(container)) })

	connectionString, err := container.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	database, err := sql.Open("pgx", connectionString)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, database.Close()) })
	_, err = database.ExecContext(ctx, `
CREATE TABLE users (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    external_id UUID NOT NULL UNIQUE,
    name TEXT NOT NULL,
    payload JSONB,
    created_at TIMESTAMPTZ NOT NULL
);
CREATE TABLE addresses (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id),
    address TEXT NOT NULL
);`)
	require.NoError(t, err)

	databaseSchema, err := postgresintrospect.Introspect(ctx, database, "public")
	require.NoError(t, err)
	require.Equal(t, schema.PostgreSQL, databaseSchema.Dialect)
	require.Len(t, databaseSchema.Tables, 2)
	require.Equal(t, "addresses", databaseSchema.Tables[0].Name)
	require.Equal(t, "users", databaseSchema.Tables[1].Name)
	require.Equal(t, []string{"user_id"}, databaseSchema.Tables[0].ForeignKeys[0].Columns)
	require.Equal(t, "users", databaseSchema.Tables[0].ForeignKeys[0].ReferencedTable)
	require.Equal(t, "uuid", databaseSchema.Tables[1].Columns[1].DBType)
	require.Equal(t, "jsonb", databaseSchema.Tables[1].Columns[3].DBType)
	require.Equal(t, "timestamptz", databaseSchema.Tables[1].Columns[4].DBType)

	generated, err := codegen.Generate(databaseSchema, codegen.Config{Package: "dbschema"})
	require.NoError(t, err)
	require.Contains(t, string(generated), "tiqq.Column[UserScope, uuid.UUID, uuid.UUID]")
	require.Contains(t, string(generated), "tiqq.Column[UserScope, sql.Null[jsontext.Value], jsontext.Value]")
	require.Contains(t, string(generated), "tiqq.Column[UserScope, time.Time, time.Time]")

	_, err = database.ExecContext(ctx, `CREATE TABLE query_probes (id BIGINT PRIMARY KEY, name TEXT NOT NULL)`)
	require.NoError(t, err)
	table := newProbeTable(tiqq.PostgreSQL)
	insert := tiqq.NewInsert[probeScope, tiqq.PostgreSQLMarker](
		table.ref, []string{"id", "name"}, []string{"id", "name"}, nil,
	).Values(table.columns.ID.Value(int64(7)), table.columns.Name.Value("Alice")).MustBuild()
	assertProbeRoundTrip(t, database, insert, table)
}

func TestMySQLIntrospectionAndCodeGeneration(t *testing.T) {
	ctx := context.Background()
	container, err := mysqlcontainer.Run(
		ctx,
		"mysql:8.4",
		mysqlcontainer.WithDatabase("tiqq"),
		mysqlcontainer.WithUsername("tiqq"),
		mysqlcontainer.WithPassword("tiqq"),
	)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, testcontainers.TerminateContainer(container)) })

	connectionString, err := container.ConnectionString(ctx, "parseTime=true")
	require.NoError(t, err)
	database, err := sql.Open("mysql", connectionString)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, database.Close()) })
	_, err = database.ExecContext(ctx, `
CREATE TABLE users (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    email VARCHAR(255) NOT NULL UNIQUE,
    payload JSON NULL,
    created_at DATETIME NOT NULL
)`)
	require.NoError(t, err)
	_, err = database.ExecContext(ctx, `
CREATE TABLE addresses (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    user_id BIGINT UNSIGNED NOT NULL,
    address VARCHAR(255) NOT NULL,
    CONSTRAINT addresses_user_fk FOREIGN KEY (user_id) REFERENCES users(id)
);`)
	require.NoError(t, err)

	databaseSchema, err := mysqlintrospect.Introspect(ctx, database, "tiqq")
	require.NoError(t, err)
	require.Equal(t, schema.MySQL, databaseSchema.Dialect)
	require.Len(t, databaseSchema.Tables, 2)
	require.Equal(t, "addresses", databaseSchema.Tables[0].Name)
	require.Equal(t, "users", databaseSchema.Tables[1].Name)
	require.Equal(t, []string{"user_id"}, databaseSchema.Tables[0].ForeignKeys[0].Columns)
	require.Equal(t, "users", databaseSchema.Tables[0].ForeignKeys[0].ReferencedTable)
	require.True(t, databaseSchema.Tables[1].Columns[0].Unsigned)
	require.True(t, databaseSchema.Tables[1].Columns[0].Identity)
	require.Equal(t, "json", databaseSchema.Tables[1].Columns[2].DBType)
	require.Equal(t, "datetime", databaseSchema.Tables[1].Columns[3].DBType)

	generated, err := codegen.Generate(databaseSchema, codegen.Config{Package: "dbschema"})
	require.NoError(t, err)
	require.Contains(t, string(generated), "tiqq.NumericColumn[UserScope, uint64, uint64, tiqq.Decimal, tiqq.Decimal]")
	require.Contains(t, string(generated), "tiqq.Column[UserScope, sql.Null[jsontext.Value], jsontext.Value]")
	require.Contains(t, string(generated), "tiqq.Column[UserScope, time.Time, time.Time]")

	_, err = database.ExecContext(ctx, `CREATE TABLE query_probes (id BIGINT PRIMARY KEY, name VARCHAR(255) NOT NULL)`)
	require.NoError(t, err)
	table := newProbeTable(tiqq.MySQL)
	insert := tiqq.NewInsert[probeScope, tiqq.MySQLMarker](
		table.ref, []string{"id", "name"}, []string{"id", "name"}, nil,
	).Values(table.columns.ID.Value(int64(7)), table.columns.Name.Value("Alice")).MustBuild()
	assertProbeRoundTrip(t, database, insert, table)
}
