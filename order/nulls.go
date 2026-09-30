package order

import (
	"fmt"
	"strings"

	"github.com/superdb/super"
	"github.com/superdb/super/sup"
)

// Nulls represents the position of nulls in an ordering of values.
type Nulls bool

var _ super.CustomMarshaler = (*Nulls)(nil)
var _ super.CustomUnmarshaler = (*Nulls)(nil)

const (
	NullsLast  Nulls = false
	NullsFirst Nulls = true
)

func (n Nulls) String() string {
	if n == NullsFirst {
		return "first"
	}
	return "last"
}

func (n Nulls) MarshalText() ([]byte, error) {
	return []byte(n.String()), nil
}

func (n *Nulls) UnmarshalText(b []byte) error {
	switch strings.ToLower(string(b)) {
	case "first":
		*n = NullsFirst
	case "last":
		*n = NullsLast
	default:
		return fmt.Errorf("unknown nulls position %q", b)
	}
	return nil
}

func (n Nulls) MarshalSuper(m *super.Marshaler) (super.Type, error) {
	return m.MarshalValue(n.String())
}

func (n *Nulls) UnmarshalSuper(u *super.Unmarshaler, val super.Value) error {
	if val.Type().ID() != super.IDString {
		return fmt.Errorf("cannot unmarshal %q into order.Nulls", sup.FormatValue(val))
	}
	return n.UnmarshalText(val.Bytes())
}
