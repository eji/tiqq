package tiqq_test

import (
	"testing"

	"github.com/eji/tiqq"
	"github.com/stretchr/testify/require"
)

func TestDecimalScan(t *testing.T) {
	tests := map[string]struct {
		value any
		want  tiqq.Decimal
	}{
		"string": {value: "1234567890.123456789", want: "1234567890.123456789"},
		"bytes":  {value: []byte("0.000000000000000001"), want: "0.000000000000000001"},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			var decimal tiqq.Decimal
			err := decimal.Scan(test.value)
			require.NoError(t, err)
			require.Equal(t, test.want, decimal)
		})
	}
}

func TestDecimalValue(t *testing.T) {
	decimal := tiqq.Decimal("1234567890.123456789")
	value, err := decimal.Value()

	require.NoError(t, err)
	require.Equal(t, "1234567890.123456789", value)
	require.Equal(t, "1234567890.123456789", decimal.String())
}
