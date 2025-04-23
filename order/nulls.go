package order

import (
	"fmt"
	"strings"

	"github.com/brimdata/super"
	"github.com/brimdata/super/sup"
)

// Nulls represents the position of nulls in an ordering of values.
type Nulls bool

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

func (n Nulls) MarshalBSUP(m *sup.MarshalBSUPContext) (super.Type, error) {
	return m.MarshalValue(n.String())
}

func (n *Nulls) UnmarshalBSUP(u *sup.UnmarshalBSUPContext, val super.Value) error {
	if val.Type().ID() != super.IDString {
		return fmt.Errorf("cannot unmarshal %q into order.Nulls", sup.FormatValue(val))
	}
	return n.UnmarshalText(val.Bytes())
}
