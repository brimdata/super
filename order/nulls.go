package order

// Nulls represents the position of nulls in an ordering of values.
type Nulls bool

const (
	NullsLast  Nulls = false
	NullsFirst Nulls = true
)
