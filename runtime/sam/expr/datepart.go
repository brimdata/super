package expr

import (
	"fmt"

	"github.com/brimdata/super"
	"github.com/brimdata/super/pkg/nano"
)

type datePartEval func(nano.Ts) int64

type datePartCall struct {
	zctx *super.Context
	fn   datePartEval
	eval Evaluator
}

func NewCallDatePart(zctx *super.Context, part string, e Evaluator) (Evaluator, error) {
	fn := lookupDatePartEval(part)
	if fn == nil {
		return nil, fmt.Errorf("unknown date part %q", part)
	}
	return &datePartCall{zctx, fn, e}, nil
}

func lookupDatePartEval(part string) datePartEval {
	switch part {
	case "day":
		return func(ts nano.Ts) int64 {
			return int64(ts.Time().Day())
		}
	case "dow", "dayofweek":
		return func(ts nano.Ts) int64 {
			return int64(ts.Time().Weekday())
		}
	case "hour":
		return func(ts nano.Ts) int64 {
			return int64(ts.Time().Hour())
		}
	case "microseconds":
		return func(ts nano.Ts) int64 {
			return int64(ts.Time().Second()*1e6 + ts.Time().Nanosecond()/1e3)
		}
	case "milliseconds":
		return func(ts nano.Ts) int64 {
			return int64(ts.Time().Second()*1e3 + ts.Time().Nanosecond()/1e6)
		}
	case "minute":
		return func(ts nano.Ts) int64 {
			return int64(ts.Time().Minute())
		}
	case "month":
		return func(ts nano.Ts) int64 {
			return int64(ts.Time().Month())
		}
	case "second":
		return func(ts nano.Ts) int64 {
			return int64(ts.Time().Second())
		}
	case "year":
		return func(ts nano.Ts) int64 {
			return int64(ts.Time().Year())
		}
	default:
		return nil
	}
}

func (d *datePartCall) Eval(ectx Context, this super.Value) super.Value {
	val := d.eval.Eval(ectx, this)
	if val.Type().ID() != super.IDTime {
		return d.zctx.WrapError("date_part: time value expected", val)
	}
	return super.NewInt64(d.fn(val.AsTime()))
}
