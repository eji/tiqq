package tiqq_test

import (
	"database/sql"
	"encoding/json/jsontext"
	"testing"
	"uuid"

	"github.com/eji/tiqq"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

type standardTypeScope struct{}

type standardTypeColumns struct {
	ID              tiqq.Column[standardTypeScope, uuid.UUID, uuid.UUID]
	Payload         tiqq.Column[standardTypeScope, jsontext.Value, jsontext.Value]
	OptionalID      tiqq.Column[standardTypeScope, sql.Null[uuid.UUID], uuid.UUID]
	OptionalPayload tiqq.Column[standardTypeScope, sql.Null[jsontext.Value], jsontext.Value]
}

type nullableStandardTypeColumns = standardTypeColumns

type standardTypeTable struct {
	ref     tiqq.TableRef
	columns standardTypeColumns
}

func (table standardTypeTable) TiqqTableInfo() tiqq.TableInfo[standardTypeColumns, nullableStandardTypeColumns] {
	return tiqq.NewTableInfo(table.ref, table.columns, table.columns, nil)
}

func newStandardTypeTable(dialect tiqq.Dialect) standardTypeTable {
	return standardTypeTable{
		ref: tiqq.NewSchemaInfo(dialect).Table("events"),
		columns: standardTypeColumns{
			ID:              tiqq.RequiredColumn[standardTypeScope, uuid.UUID]("events", "id"),
			Payload:         tiqq.RequiredColumn[standardTypeScope, jsontext.Value]("events", "payload"),
			OptionalID:      tiqq.NullableColumn[standardTypeScope, uuid.UUID]("events", "optional_id"),
			OptionalPayload: tiqq.NullableColumn[standardTypeScope, jsontext.Value]("events", "optional_payload"),
		},
	}
}

func TestStatementArgsNormalizeStandardTypes(t *testing.T) {
	table := newStandardTypeTable(tiqq.PostgreSQL)
	id := uuid.MustParse("01890a5d-ac96-774b-bcce-b302099a8057")
	payload := jsontext.Value(`{"name":"Alice"}`)
	statement := tiqq.NewTableQuery(table).
		Where(table.columns.ID.Eq(id), table.columns.Payload.Eq(payload)).
		Select(table.columns.ID).
		MustBuild()

	require.Equal(t, []any{id.String(), []byte(`{"name":"Alice"}`)}, statement.Args())
}

func TestRowGetUUIDDriverRepresentations(t *testing.T) {
	table := newStandardTypeTable(tiqq.PostgreSQL)
	id := uuid.MustParse("01890a5d-ac96-774b-bcce-b302099a8057")
	statement := tiqq.NewTableQuery(table).Select(table.columns.ID).MustBuild()
	tests := map[string]struct {
		raw any
	}{
		"UUID":         {raw: id},
		"array":        {raw: [16]byte(id)},
		"binary bytes": {raw: append([]byte(nil), id[:]...)},
		"text bytes":   {raw: []byte(id.String())},
		"string":       {raw: id.String()},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			row, err := tiqq.ScanRow(scannerFunc(func(destinations ...any) error {
				*destinations[0].(*any) = test.raw
				return nil
			}), statement)
			got, getErr := row.Get(table.columns.ID)

			require.NoError(t, err)
			require.NoError(t, getErr)
			require.Equal(t, id, got)
		})
	}
}

func TestRowGetJSONDriverRepresentations(t *testing.T) {
	table := newStandardTypeTable(tiqq.PostgreSQL)
	want := jsontext.Value(`{"name":"Alice"}`)
	statement := tiqq.NewTableQuery(table).Select(table.columns.Payload).MustBuild()
	tests := map[string]struct {
		raw any
	}{
		"JSON value": {raw: want},
		"bytes":      {raw: []byte(want)},
		"string":     {raw: string(want)},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			row, err := tiqq.ScanRow(scannerFunc(func(destinations ...any) error {
				*destinations[0].(*any) = test.raw
				return nil
			}), statement)
			got, getErr := row.Get(table.columns.Payload)

			require.NoError(t, err)
			require.NoError(t, getErr)
			require.Equal(t, want, got)
		})
	}
}

func TestRowGetNullableStandardTypes(t *testing.T) {
	table := newStandardTypeTable(tiqq.PostgreSQL)
	id := uuid.MustParse("01890a5d-ac96-774b-bcce-b302099a8057")
	payload := jsontext.Value(`{"active":true}`)
	statement := tiqq.NewTableQuery(table).
		Select(table.columns.OptionalID, table.columns.OptionalPayload).
		MustBuild()
	row, err := tiqq.ScanRow(scannerFunc(func(destinations ...any) error {
		*destinations[0].(*any) = id.String()
		*destinations[1].(*any) = []byte(payload)
		return nil
	}), statement)
	gotID, idErr := row.Get(table.columns.OptionalID)
	gotPayload, payloadErr := row.Get(table.columns.OptionalPayload)

	require.NoError(t, err)
	require.NoError(t, idErr)
	require.NoError(t, payloadErr)
	require.Equal(t, sql.Null[uuid.UUID]{V: id, Valid: true}, gotID)
	require.Equal(t, sql.Null[jsontext.Value]{V: payload, Valid: true}, gotPayload)
}

func TestRowGetRejectsInvalidStandardTypes(t *testing.T) {
	table := newStandardTypeTable(tiqq.PostgreSQL)
	tests := map[string]struct {
		selection tiqq.Selection
		raw       any
		get       func(tiqq.Row) error
		want      string
	}{
		"UUID": {
			selection: table.columns.ID,
			raw:       "not-a-uuid",
			get: func(row tiqq.Row) error {
				_, err := row.Get(table.columns.ID)
				return err
			},
			want: "tiqq: column events.id: invalid UUID",
		},
		"JSON": {
			selection: table.columns.Payload,
			raw:       []byte(`{"invalid"`),
			get: func(row tiqq.Row) error {
				_, err := row.Get(table.columns.Payload)
				return err
			},
			want: "tiqq: column events.payload: invalid JSON",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			statement := tiqq.NewTableQuery(table).Select(test.selection).MustBuild()
			row, err := tiqq.ScanRow(scannerFunc(func(destinations ...any) error {
				*destinations[0].(*any) = test.raw
				return nil
			}), statement)

			require.NoError(t, err)
			require.ErrorContains(t, test.get(row), test.want)
		})
	}
}

func TestStandardTypesRoundTripThroughDatabaseSQL(t *testing.T) {
	database, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = database.Close() })
	_, err = database.Exec(`CREATE TABLE events (id UUID PRIMARY KEY, payload JSON NOT NULL)`)
	require.NoError(t, err)

	table := newStandardTypeTable(tiqq.SQLite)
	id := uuid.MustParse("01890a5d-ac96-774b-bcce-b302099a8057")
	payload := jsontext.Value(`{"name":"Alice"}`)
	insert := tiqq.NewInsert[standardTypeScope, tiqq.SQLiteMarker](
		table.ref, []string{"id", "payload"}, []string{"id", "payload"}, [][]string{{"id"}},
	).Values(table.columns.ID.Value(id), table.columns.Payload.Value(payload)).MustBuild()
	_, err = database.Exec(insert.SQL(), insert.Args()...)
	require.NoError(t, err)

	selectStatement := tiqq.NewTableQuery(table).
		Where(table.columns.ID.Eq(id)).
		Select(table.columns.ID, table.columns.Payload).
		MustBuild()
	row, err := tiqq.ScanRow(
		database.QueryRow(selectStatement.SQL(), selectStatement.Args()...),
		selectStatement,
	)
	gotID, idErr := row.Get(table.columns.ID)
	gotPayload, payloadErr := row.Get(table.columns.Payload)

	require.NoError(t, err)
	require.NoError(t, idErr)
	require.NoError(t, payloadErr)
	require.Equal(t, id, gotID)
	require.Equal(t, payload, gotPayload)
}
