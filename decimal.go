package tiqq

import (
	"database/sql/driver"
	"fmt"
)

// Decimal is an exact PostgreSQL numeric/decimal representation. It retains
// the database text representation and never converts through floating point.
type Decimal string

func (decimal *Decimal) Scan(value any) error {
	switch value := value.(type) {
	case string:
		*decimal = Decimal(value)
		return nil
	case []byte:
		*decimal = Decimal(string(value))
		return nil
	case nil:
		return fmt.Errorf("tiqq: cannot scan NULL into Decimal")
	default:
		return fmt.Errorf("tiqq: cannot scan %T into Decimal", value)
	}
}

func (decimal Decimal) Value() (driver.Value, error) {
	return string(decimal), nil
}

func (decimal Decimal) String() string { return string(decimal) }
