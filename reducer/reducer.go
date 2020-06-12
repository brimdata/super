package reducer

//XXX in new model, need to do a semantic check on the reducers since they
// are compiled at runtime and you don't want to run a long time then catch
// the error that could have been caught earlier

import (
	"errors"

	"github.com/brimsec/zq/zng"
	"github.com/brimsec/zq/zng/resolver"
)

var (
	ErrBadValue = errors.New("bad value")
)

type Interface interface {
	Consume(*zng.Record)
	Result() zng.Value
}

type Decomposable interface {
	Interface
	ConsumePart(zng.Value) error
	ResultPart(*resolver.Context) (zng.Value, error)
}

type Stats struct {
	TypeMismatch  int64
	FieldNotFound int64
}

type Reducer struct {
	Stats
}
