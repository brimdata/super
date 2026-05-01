package function

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/pkg/nano"
)

type DatePart struct {
	sctx *super.Context
}

func NewDatePart(sctx *super.Context) *DatePart {
	return &DatePart{sctx}
}

func (d *DatePart) Call(args []super.Value) super.Value {
	args = underAll(args)
	part, time := args[0], args[1]
	if part.IsNull() || time.IsNull() {
		return super.Null
	}
	if part.Type().ID() != super.IDString {
		return d.sctx.WrapError("date_part: string value required for part argument", args[0])
	}
	if time.Type().ID() != super.IDTime {
		return d.sctx.WrapError("date_part: time value required for time argument", args[1])
	}
	fn := lookupDatePartEval(part.AsString())
	if fn == nil {
		return d.sctx.WrapError("date_part: unsupported part name", part)
	}
	return super.NewInt64(fn(time.AsTime()))
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
