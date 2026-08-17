package tiqq

// Order is a typed ORDER BY expression.
type Order struct {
	column    columnRef
	direction string
}

// Asc orders rows by this column in ascending order.
func (c Column[S, V, C]) Asc() Order {
	return Order{column: c.ref, direction: "ASC"}
}

// Desc orders rows by this column in descending order.
func (c Column[S, V, C]) Desc() Order {
	return Order{column: c.ref, direction: "DESC"}
}

// Asc orders rows by this aggregate in ascending order.
func (a Aggregate[S, V, C]) Asc() Order {
	return Order{column: a.ref, direction: "ASC"}
}

// Desc orders rows by this aggregate in descending order.
func (a Aggregate[S, V, C]) Desc() Order {
	return Order{column: a.ref, direction: "DESC"}
}
