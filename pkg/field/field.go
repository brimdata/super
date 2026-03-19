package field

import (
	"slices"
	"strings"
)

type Path []string

func (p Path) String() string { return string(p.AppendTo(nil)) }

// AppendTo appends the string representation of the path to byte slice b.
func (p Path) AppendTo(b []byte) []byte {
	if len(p) == 0 {
		return append(b, "this"...)
	}
	for i, s := range p {
		if i > 0 {
			b = append(b, '.')
		}
		b = append(b, s...)
	}
	return b
}

func (p Path) Leaf() string {
	return p[len(p)-1]
}

func (p Path) Equal(to Path) bool {
	return slices.Equal(p, to)
}

func (p Path) IsEmpty() bool {
	return len(p) == 0
}

func (p Path) HasStrictPrefix(prefix Path) bool {
	return len(p) > len(prefix) && prefix.Equal(p[:len(prefix)])
}

func (p Path) HasPrefix(prefix Path) bool {
	return len(p) >= len(prefix) && prefix.Equal(p[:len(prefix)])
}

func (p Path) In(list List) bool {
	return list.Has(p)
}

func (p Path) HasPrefixIn(set []Path) bool {
	return slices.ContainsFunc(set, p.HasPrefix)
}

func Dotted(s string) Path {
	return strings.Split(s, ".")
}

func DottedList(s string) List {
	var fields List
	for path := range strings.SplitSeq(s, ",") {
		fields = append(fields, Dotted(path))
	}
	return fields
}

type List []Path

func (l List) String() string { return string(l.AppendTo(nil)) }

// AppendTo appends the string representation of the list to byte slice b.
func (l List) AppendTo(b []byte) []byte {
	for i, p := range l {
		if i > 0 {
			b = append(b, ',')
		}
		b = p.AppendTo(b)
	}
	return b
}

func (l List) Has(in Path) bool {
	return slices.ContainsFunc(l, in.Equal)
}

func (l List) Equal(to List) bool {
	return slices.EqualFunc(l, to, func(a, b Path) bool {
		return a.Equal(b)
	})
}
