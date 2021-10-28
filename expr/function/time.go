package function

import (
	"github.com/brimdata/zed"
	"github.com/brimdata/zed/expr/coerce"
	"github.com/brimdata/zed/expr/result"
	"github.com/brimdata/zed/pkg/nano"
)

type trunc struct {
	result.Buffer
}

func (t *trunc) Call(args []zed.Value) (zed.Value, error) {
	tsArg := args[0]
	binArg := args[1]
	if tsArg.Bytes == nil || binArg.Bytes == nil {
		return zed.Value{zed.TypeTime, nil}, nil
	}
	ts, ok := coerce.ToTime(tsArg)
	if !ok {
		return badarg("trunc")
	}
	var bin nano.Duration
	if binArg.Type == zed.TypeDuration {
		var err error
		bin, err = zed.DecodeDuration(binArg.Bytes)
		if err != nil {
			return zed.Value{}, err
		}
	} else {
		d, ok := coerce.ToInt(binArg)
		if !ok {
			return badarg("trunc")
		}
		bin = nano.Duration(d) * nano.Second
	}
	return zed.Value{zed.TypeTime, t.Time(nano.Ts(ts.Trunc(bin)))}, nil
}
