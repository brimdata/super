package zng

import (
	"github.com/brimdata/zed/zcode"
)

type TypeOfNull struct{}

func (t *TypeOfNull) ID() int {
	return IdNull
}

func (t *TypeOfNull) String() string {
	return "null"
}

func (t *TypeOfNull) Marshal(zv zcode.Bytes) (interface{}, error) {
	return nil, nil
}

func (t *TypeOfNull) ZSON() string {
	return "null"
}

func (t *TypeOfNull) ZSONOf(zv zcode.Bytes) string {
	return "null"
}
