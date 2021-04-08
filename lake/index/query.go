package index

import (
	"strings"

	"github.com/brimdata/zed/zio/tzngio"
	"github.com/brimdata/zed/zqe"
	"github.com/brimdata/zed/zson"
	"github.com/segmentio/ksuid"
)

type DefLookup struct {
	DefID  ksuid.KSUID
	Values []string
}

type Query struct {
	Name   string
	Field  string
	Type   string
	Values []string
}

func ParseQuery(name string, patterns []string) (Query, error) {
	if len(patterns) == 0 {
		return Query{}, zqe.E(zqe.Invalid, "no search patterns")
	}
	if name != "" {
		return Query{
			Name:   name,
			Values: patterns,
		}, nil
	}
	if len(patterns) != 1 {
		return Query{}, zqe.E(zqe.Invalid, "standard index supports exactly one search pattern")
	}
	in := patterns[0]

	v := strings.Split(in, "=")
	if len(v) != 2 {
		return Query{}, zqe.E(zqe.Invalid, "malformed standard index query")
	}
	q := Query{Values: []string{v[1]}}
	path := v[0]
	if path[0] == ':' {
		typ, err := zson.NewContext().LookupByName(path[1:])
		if err != nil {
			return Query{}, err
		}
		//XXX should use zson
		q.Type = tzngio.TypeString(typ)
	} else {
		q.Field = path
	}
	return q, nil
}

func (q Query) Matches(r Rule) bool {
	switch r.Kind {
	case RuleZQL:
		return q.Name == r.Name
	case RuleType:
		return q.Type == r.Type
	case RuleField:
		return q.Field == r.Field
	}
	return false
}
