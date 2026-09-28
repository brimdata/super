package order

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/brimdata/super"
)

type Direction int

var _ super.CustomMarshaler = (*Direction)(nil)
var _ super.CustomUnmarshaler = (*Direction)(nil)

const (
	Down    Direction = -1
	Up      Direction = 1
	Unknown Direction = 0
)

func ParseDirection(s string) (Direction, error) {
	switch s {
	case "asc":
		return Up, nil
	case "desc":
		return Down, nil
	case "unknown", "dontcare", "":
		return Unknown, nil
	default:
		return Unknown, fmt.Errorf("unknown direction string: %s (should be asc, desc, unknown, or dontcare)", s)
	}
}

func (d Direction) Which() (Which, bool) {
	switch d {
	case Down:
		return Desc, true
	case Up:
		return Asc, true
	default:
		return Asc, false
	}
}

func (d Direction) HasOrder(which Which) bool {
	switch d {
	case Up:
		return which == Asc
	case Down:
		return which == Desc
	default:
		return false
	}
}

func (d Direction) String() string {
	switch {
	case d < 0:
		return "desc"
	case d > 0:
		return "asc"
	default:
		return "unknown"
	}
}

func (d *Direction) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	switch strings.ToLower(s) {
	case "asc":
		*d = Up
	case "desc":
		*d = Down
	default:
		*d = Unknown
	}
	return nil
}

func (d Direction) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.String())
}

func (d Direction) MarshalSuper(m *super.Marshaler) (super.Type, error) {
	return m.MarshalValue(d.String())
}

func (d *Direction) UnmarshalSuper(u *super.Unmarshaler, val super.Value) error {
	dir, err := ParseDirection(string(val.Bytes()))
	if err != nil {
		return err
	}
	*d = dir
	return nil
}
