//go:build integration

package integration_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/eji/tiqq"
	"github.com/eji/tiqq/codegen"
	mysqlintrospect "github.com/eji/tiqq/introspect/mysql"
	postgresintrospect "github.com/eji/tiqq/introspect/postgres"
	mysqldialect "github.com/eji/tiqq/mysql"
	postgresdialect "github.com/eji/tiqq/postgres"
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
    balance NUMERIC(12,2) NOT NULL,
    payload JSONB,
    created_at TIMESTAMPTZ NOT NULL
);
CREATE TABLE addresses (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id),
    address TEXT NOT NULL
);
CREATE SCHEMA identity;
CREATE TABLE identity.accounts (
    id BIGINT PRIMARY KEY
);
CREATE TABLE audit_logs (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    account_id BIGINT NOT NULL REFERENCES identity.accounts(id)
);`)
	require.NoError(t, err)

	databaseSchema, err := postgresintrospect.Introspect(ctx, database, "public")
	require.NoError(t, err)
	require.Equal(t, schema.PostgreSQL, databaseSchema.Dialect)
	require.Len(t, databaseSchema.Tables, 3)
	require.Equal(t, "addresses", databaseSchema.Tables[0].Name)
	require.Equal(t, "audit_logs", databaseSchema.Tables[1].Name)
	require.Equal(t, "users", databaseSchema.Tables[2].Name)
	require.Equal(t, []string{"user_id"}, databaseSchema.Tables[0].ForeignKeys[0].Columns)
	require.Equal(t, "users", databaseSchema.Tables[0].ForeignKeys[0].ReferencedTable)
	require.Equal(t, "identity", databaseSchema.Tables[1].ForeignKeys[0].ReferencedSchema)
	require.Equal(t, "accounts", databaseSchema.Tables[1].ForeignKeys[0].ReferencedTable)
	require.Equal(t, "uuid", databaseSchema.Tables[2].Columns[1].DBType)
	require.Equal(t, "numeric", databaseSchema.Tables[2].Columns[3].DBType)
	require.Equal(t, "jsonb", databaseSchema.Tables[2].Columns[4].DBType)
	require.Equal(t, "timestamptz", databaseSchema.Tables[2].Columns[5].DBType)

	generated, err := codegen.Generate(databaseSchema, codegen.Config{Package: "dbschema"})
	require.NoError(t, err)
	require.Contains(t, string(generated), "tiqq.Column[UserScope, uuid.UUID, uuid.UUID]")
	require.Contains(t, string(generated), "tiqq.Column[UserScope, sql.Null[jsontext.Value], jsontext.Value]")
	require.Contains(t, string(generated), "tiqq.Column[UserScope, time.Time, time.Time]")
	require.NotContains(t, string(generated), `ReferencedTable: "accounts"`)
	_, err = database.Exec(`INSERT INTO users (external_id, name, balance, payload, created_at) VALUES
('01890a5d-ac96-774b-bcce-b302099a8057', 'Typed', 12.34, '{"active":true}', '2026-08-18T00:00:00Z')`)
	require.NoError(t, err)
	var externalID, balance string
	var payload []byte
	var createdAt any
	var missingAddress sql.NullString
	err = database.QueryRow(`SELECT u.external_id, u.balance, u.payload, u.created_at, a.address
FROM users u LEFT JOIN addresses a ON u.id = a.user_id WHERE u.name = 'Typed'`).
		Scan(&externalID, &balance, &payload, &createdAt, &missingAddress)
	require.NoError(t, err)
	require.Equal(t, "01890a5d-ac96-774b-bcce-b302099a8057", externalID)
	require.Equal(t, "12.34", balance)
	require.JSONEq(t, `{"active":true}`, string(payload))
	require.IsType(t, time.Time{}, createdAt)
	require.False(t, missingAddress.Valid)

	_, err = database.ExecContext(ctx, `CREATE TABLE query_probes (id BIGINT PRIMARY KEY, name TEXT NOT NULL)`)
	require.NoError(t, err)
	table := newProbeTable(tiqq.PostgreSQL)
	insert := tiqq.NewInsert[probeScope, tiqq.PostgreSQLMarker](
		table.ref, []string{"id", "name"}, []string{"id", "name"}, nil,
	).Values(table.columns.ID.Value(int64(7)), table.columns.Name.Value("Alice")).MustBuild()
	assertProbeRoundTrip(t, database, insert, table)
	returning := tiqq.NewInsert[probeScope, tiqq.PostgreSQLMarker](
		table.ref, []string{"id", "name"}, []string{"id", "name"}, nil,
	).Values(
		table.columns.ID.Value(int64(8)), table.columns.Name.Value("Bob"),
	).Returning(table.columns.ID, table.columns.Name).MustBuild()
	returnedRow, err := tiqq.ScanRow(
		database.QueryRow(returning.SQL(), returning.Args()...), returning,
	)
	returnedID, idErr := returnedRow.Get(table.columns.ID)
	returnedName, nameErr := returnedRow.Get(table.columns.Name)
	require.NoError(t, err)
	require.NoError(t, idErr)
	require.NoError(t, nameErr)
	require.Equal(t, int64(8), returnedID)
	require.Equal(t, "Bob", returnedName)
	upsert := postgresdialect.NewInsert(tiqq.NewInsert[probeScope, tiqq.PostgreSQLMarker](
		table.ref, []string{"id", "name"}, []string{"id", "name"}, [][]string{{"id"}},
	)).Values(table.columns.ID.Value(int64(7)), table.columns.Name.Value("Updated")).
		OnConflict(table.columns.ID).DoUpdate(postgresdialect.Excluded(table.columns.Name)).MustBuild()
	_, err = database.Exec(upsert.SQL(), upsert.Args()...)
	require.NoError(t, err)
	updated := tiqq.NewUpdate[probeScope](table).Set(table.columns.Name.To("Final")).
		Where(table.columns.ID.Eq(int64(7))).Returning(table.columns.Name).MustBuild()
	updatedRow, err := tiqq.ScanRow(database.QueryRow(updated.SQL(), updated.Args()...), updated)
	updatedName, nameErr := updatedRow.Get(table.columns.Name)
	require.NoError(t, err)
	require.NoError(t, nameErr)
	require.Equal(t, "Final", updatedName)
	deleted := tiqq.NewDelete[probeScope](table).Where(table.columns.ID.Eq(int64(8))).
		Returning(table.columns.ID).MustBuild()
	deletedRow, err := tiqq.ScanRow(database.QueryRow(deleted.SQL(), deleted.Args()...), deleted)
	deletedID, deleteErr := deletedRow.Get(table.columns.ID)
	require.NoError(t, err)
	require.NoError(t, deleteErr)
	require.Equal(t, int64(8), deletedID)
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
    balance DECIMAL(12,2) NOT NULL,
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
	require.Equal(t, "decimal", databaseSchema.Tables[1].Columns[2].DBType)
	require.Equal(t, "json", databaseSchema.Tables[1].Columns[3].DBType)
	require.Equal(t, "datetime", databaseSchema.Tables[1].Columns[4].DBType)

	generated, err := codegen.Generate(databaseSchema, codegen.Config{Package: "dbschema"})
	require.NoError(t, err)
	require.Contains(t, string(generated), "tiqq.NumericColumn[UserScope, uint64, uint64, tiqq.Decimal, tiqq.Decimal]")
	require.Contains(t, string(generated), "tiqq.Column[UserScope, sql.Null[jsontext.Value], jsontext.Value]")
	require.Contains(t, string(generated), "tiqq.Column[UserScope, time.Time, time.Time]")
	_, err = database.Exec(`INSERT INTO users (email, balance, payload, created_at) VALUES
('typed@example.com', 12.34, '{"active":true}', '2026-08-18 00:00:00')`)
	require.NoError(t, err)
	var balance string
	var payload []byte
	var createdAt any
	var missingAddress sql.NullString
	err = database.QueryRow(`SELECT u.balance, u.payload, u.created_at, a.address
FROM users u LEFT JOIN addresses a ON u.id = a.user_id WHERE u.email = 'typed@example.com'`).
		Scan(&balance, &payload, &createdAt, &missingAddress)
	require.NoError(t, err)
	require.Equal(t, "12.34", balance)
	require.JSONEq(t, `{"active":true}`, string(payload))
	require.IsType(t, time.Time{}, createdAt)
	require.False(t, missingAddress.Valid)

	_, err = database.ExecContext(ctx, `CREATE TABLE query_probes (id BIGINT PRIMARY KEY, name VARCHAR(255) NOT NULL)`)
	require.NoError(t, err)
	table := newProbeTable(tiqq.MySQL)
	insert := tiqq.NewInsert[probeScope, tiqq.MySQLMarker](
		table.ref, []string{"id", "name"}, []string{"id", "name"}, nil,
	).Values(table.columns.ID.Value(int64(7)), table.columns.Name.Value("Alice")).MustBuild()
	assertProbeRoundTrip(t, database, insert, table)
	upsert := mysqldialect.NewInsert(tiqq.NewInsert[probeScope, tiqq.MySQLMarker](
		table.ref, []string{"id", "name"}, []string{"id", "name"}, [][]string{{"id"}},
	)).Values(table.columns.ID.Value(int64(7)), table.columns.Name.Value("Updated")).
		OnDuplicateKey().DoUpdate(mysqldialect.Inserted(table.columns.Name)).MustBuild()
	_, err = database.Exec(upsert.SQL(), upsert.Args()...)
	require.NoError(t, err)
	updated := tiqq.NewUpdate[probeScope](table).Set(table.columns.Name.To("Final")).
		Where(table.columns.ID.Eq(int64(7))).MustBuild()
	_, err = database.Exec(updated.SQL(), updated.Args()...)
	require.NoError(t, err)
	deleted := tiqq.NewDelete[probeScope](table).Where(table.columns.ID.Eq(int64(7))).MustBuild()
	_, err = database.Exec(deleted.SQL(), deleted.Args()...)
	require.NoError(t, err)
	var remaining int
	err = database.QueryRow(`SELECT COUNT(*) FROM query_probes WHERE id = 7`).Scan(&remaining)
	require.NoError(t, err)
	require.Zero(t, remaining)
}
