package order

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/brimdata/super"
	"github.com/brimdata/super/sup"
)

type Which bool

const (
	Asc  Which = false
	Desc Which = true
)

func Parse(s string) (Which, error) {
	switch strings.ToLower(s) {
	case "asc":
		return Asc, nil
	case "desc":
		return Desc, nil
	default:
		return false, fmt.Errorf("unknown order: %s", s)
	}
}

func (w Which) String() string {
	if w == Desc {
		return "desc"
	}
	return "asc"
}

func (w Which) Direction() Direction {
	if w == Desc {
		return Down
	}
	return Up
}

func (w Which) MarshalJSON() ([]byte, error) {
	return json.Marshal(w.String())
}

func (w *Which) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	switch s {
	case "asc":
		*w = Asc
	case "desc":
		*w = Desc
	default:
		return fmt.Errorf("unknown order: %s", s)
	}
	return nil
}

func (w Which) MarshalBSUP(m *sup.MarshalBSUPContext) (super.Type, error) {
	return m.MarshalValue(w.String())
}

func (w *Which) UnmarshalBSUP(u *sup.UnmarshalBSUPContext, val super.Value) error {
	which, err := Parse(string(val.Bytes()))
	if err != nil {
		return err
	}
	*w = which
	return nil
}
