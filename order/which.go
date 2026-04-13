package order

import (
	"fmt"
	"strings"
)

type Which string

const (
	Asc  Which = "asc"
	Desc Which = "desc"
)

func (w Which) String() string {
	return string(w)
}

func (w Which) Direction() Direction {
	if w == Desc {
		return Down
	}
	return Up
}

func Parse(s string) (Which, error) {
	switch strings.ToLower(s) {
	case "asc":
		return Asc, nil
	case "desc":
		return Desc, nil
	default:
		return "", fmt.Errorf("unknown order: %s", s)
	}
}

func (w Which) Reverse() Which {
	if w == Asc {
		return Desc
	}
	return Asc
}

// NullsMax returns the Nulls value corresponding to w and nullsMax.
func (w Which) NullsMax(nullsMax bool) Nulls {
	if w == Asc && nullsMax || w == Desc && !nullsMax {
		return NullsLast
	}
	return NullsFirst
}
