package function

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/pkg/nano"
	"github.com/brimdata/super/runtime/sam/expr/coerce"
	"github.com/lestrrat-go/strftime"
)

// https://github.com/brimdata/super/blob/main/docs/language/functions.md#now
type Now struct{}

func (n *Now) Call(_ super.Allocator, _ []super.Value) super.Value {
	return super.NewTime(nano.Now())
}

// https://github.com/brimdata/super/blob/main/docs/language/functions.md#bucket
type Bucket struct {
	name string
	zctx *super.Context
}

func (b *Bucket) Call(_ super.Allocator, args []super.Value) super.Value {
	tsArg := args[0]
	binArg := args[1]
	if tsArg.IsNull() || binArg.IsNull() {
		return super.NullTime
	}
	var bin nano.Duration
	if binArg.Type() == super.TypeDuration {
		bin = nano.Duration(binArg.Int())
	} else {
		d, ok := coerce.ToInt(binArg)
		if !ok {
			return b.zctx.WrapError(b.name+": second argument is not a duration or number", binArg)
		}
		bin = nano.Duration(d) * nano.Second
	}
	if super.TypeUnder(tsArg.Type()) == super.TypeDuration {
		dur := nano.Duration(tsArg.Int())
		return super.NewDuration(dur.Trunc(bin))
	}
	v, ok := coerce.ToInt(tsArg)
	if !ok {
		return b.zctx.WrapError(b.name+": first argument is not a time", tsArg)
	}
	return super.NewTime(nano.Ts(v).Trunc(bin))
}

// https://github.com/brimdata/super/blob/main/docs/language/functions.md#strftime
type Strftime struct {
	zctx      *super.Context
	formatter *strftime.Strftime
}

func (s *Strftime) Call(_ super.Allocator, args []super.Value) super.Value {
	formatArg, timeArg := args[0], args[1]
	if !formatArg.IsString() {
		return s.zctx.WrapError("strftime: string value required for format arg", formatArg)
	}
	if super.TypeUnder(timeArg.Type()) != super.TypeTime {
		return s.zctx.WrapError("strftime: time value required for time arg", args[1])
	}
	format := formatArg.AsString()
	if s.formatter == nil || s.formatter.Pattern() != format {
		var err error
		if s.formatter, err = strftime.New(format); err != nil {
			return s.zctx.WrapError("strftime: "+err.Error(), formatArg)
		}
	}
	if timeArg.IsNull() {
		return super.NullString
	}
	out := s.formatter.FormatString(timeArg.AsTime().Time())
	return super.NewString(out)
}
