package function

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/pkg/nano"
)

type DatePart struct {
	zctx *super.Context
}

func NewDatePart(zctx *super.Context) *DatePart {
	return &DatePart{zctx}
}

func (d *DatePart) Call(_ super.Allocator, args []super.Value) super.Value {
	if args[0].Type().ID() != super.IDString {
		return d.zctx.WrapError("date_part: string value required for part argument", args[0])
	}
	if args[1].Type().ID() != super.IDTime {
		return d.zctx.WrapError("date_part: time value required for time argument", args[1])
	}
	fn := lookupDatePartEval(args[0].AsString())
	if fn == nil {
		return d.zctx.WrapError("date_part: unsupported part name", args[0])
	}
	return super.NewInt64(fn(args[1].AsTime()))
}

func lookupDatePartEval(part string) func(nano.Ts) int64 {
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
