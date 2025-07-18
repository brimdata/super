package function

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/pkg/nano"
	"github.com/lestrrat-go/strftime"
)

// https://github.com/brimdata/super/blob/main/docs/language/functions.md#now
type Now struct{}

func (n *Now) Call(_ []super.Value) super.Value {
	return super.NewTime(nano.Now())
}

// https://github.com/brimdata/super/blob/main/docs/language/functions.md#bucket
type Bucket struct {
	name string
	sctx *super.Context
}

func (b *Bucket) Call(args []super.Value) super.Value {
	args = underAll(args)
	tsArg := args[0]
	binArg := args[1]
	tsArgID := tsArg.Type().ID()
	if tsArgID != super.IDDuration && tsArgID != super.IDTime {
		return b.sctx.WrapError(b.name+": first argument is not a time or duration", tsArg)
	}
	if binArg.Type().ID() != super.IDDuration {
		return b.sctx.WrapError(b.name+": second argument is not a duration", binArg)
	}
	if tsArg.IsNull() || binArg.IsNull() {
		if tsArgID == super.IDDuration {
			return super.NullDuration
		}
		return super.NullTime
	}
	bin := nano.Duration(binArg.Int())
	if tsArgID == super.IDDuration {
		dur := nano.Duration(tsArg.Int())
		if bin != 0 {
			dur = dur.Trunc(bin)
		}
		return super.NewDuration(dur)
	}
	ts := nano.Ts(tsArg.Int())
	if bin != 0 {
		ts = ts.Trunc(bin)
	}
	return super.NewTime(ts)
}

// https://github.com/brimdata/super/blob/main/docs/language/functions.md#strftime
type Strftime struct {
	sctx      *super.Context
	formatter *strftime.Strftime
}

func (s *Strftime) Call(args []super.Value) super.Value {
	formatArg, timeArg := args[0], args[1]
	if !formatArg.IsString() {
		return s.sctx.WrapError("strftime: string value required for format arg", formatArg)
	}
	if super.TypeUnder(timeArg.Type()) != super.TypeTime {
		return s.sctx.WrapError("strftime: time value required for time arg", args[1])
	}
	format := formatArg.AsString()
	if s.formatter == nil || s.formatter.Pattern() != format {
		var err error
		if s.formatter, err = strftime.New(format); err != nil {
			return s.sctx.WrapError("strftime: "+err.Error(), formatArg)
		}
	}
	if timeArg.IsNull() {
		return super.NullString
	}
	out := s.formatter.FormatString(timeArg.AsTime().Time())
	return super.NewString(out)
}
