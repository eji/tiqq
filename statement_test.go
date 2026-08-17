package tiqq_test

import (
	"database/sql"
	"errors"
	"testing"

	"github.com/eji/tiqq"
	"github.com/stretchr/testify/require"
)

type scannerFunc func(destinations ...any) error

func (scanner scannerFunc) Scan(destinations ...any) error {
	return scanner(destinations...)
}

func TestScanRow(t *testing.T) {
	j := prototypeJoin()
	stmt := j.Select(j.Left().ID, j.Right().Address).MustBuild()
	row, err := tiqq.ScanRow(scannerFunc(func(destinations ...any) error {
		*destinations[0].(*any) = int64(7)
		*destinations[1].(*any) = "Tokyo"
		return nil
	}), stmt)

	require.NoError(t, err)
	require.Equal(t, int64(7), row.Get(j.Left().ID))
	require.Equal(t, sql.Null[string]{V: "Tokyo", Valid: true}, row.Get(j.Right().Address))
}

func TestScanRowPreservesNull(t *testing.T) {
	j := prototypeJoin()
	stmt := j.Select(j.Right().Address).MustBuild()
	row, err := tiqq.ScanRow(scannerFunc(func(destinations ...any) error {
		*destinations[0].(*any) = nil
		return nil
	}), stmt)

	require.NoError(t, err)
	require.Equal(t, sql.Null[string]{}, row.Get(j.Right().Address))
}

func TestScanRowConvertsDriverInteger(t *testing.T) {
	j := prototypeJoin()
	stmt := j.Select(j.Right().ID).MustBuild()
	row, err := tiqq.ScanRow(scannerFunc(func(destinations ...any) error {
		*destinations[0].(*any) = int64(12)
		return nil
	}), stmt)

	require.NoError(t, err)
	require.Equal(t, sql.Null[int64]{V: 12, Valid: true}, row.Get(j.Right().ID))
}

func TestScanRowWrapsScannerError(t *testing.T) {
	j := prototypeJoin()
	stmt := j.Select(j.Left().ID).MustBuild()
	_, err := tiqq.ScanRow(scannerFunc(func(destinations ...any) error {
		return errors.New("driver failure")
	}), stmt)

	require.EqualError(t, err, "tiqq: scan row: driver failure")
}
