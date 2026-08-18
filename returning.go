package tiqq

import (
	"fmt"
	"strings"
)

func selections(columns []Selection) []columnRef {
	result := make([]columnRef, len(columns))
	for index, column := range columns {
		result[index] = column.selection()
	}
	return result
}

func validateReturning(renderer sqlRenderer, table TableRef, columns []columnRef, requested bool) error {
	if !requested {
		return nil
	}
	if renderer.name() == "mysql" {
		return fmt.Errorf("tiqq: mysql does not support RETURNING")
	}
	if len(columns) == 0 {
		return fmt.Errorf("tiqq: RETURNING requires at least one column")
	}
	allowed := map[string]bool{table.qualifier(): true}
	if err := validateColumns("RETURNING", columns, allowed); err != nil {
		return err
	}
	return validateNoAggregates("RETURNING", columns)
}

func renderReturning(renderer sqlRenderer, builder *strings.Builder, columns []columnRef) {
	if len(columns) == 0 {
		return
	}
	builder.WriteString(" RETURNING ")
	for index, column := range columns {
		if index > 0 {
			builder.WriteString(", ")
		}
		builder.WriteString(renderer.quoteIdentifier(column.name))
	}
}
