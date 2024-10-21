package function

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/pkg/nano"
	"github.com/brimdata/super/vector"
	"github.com/lestrrat-go/strftime"
)

// https://github.com/brimdata/super/blob/main/docs/language/functions.md#strftime
type Strftime struct {
	zctx *super.Context
}

func (s *Strftime) Call(args ...vector.Any) vector.Any {
	args = underAll(args)
	formatVec, timeVec := args[0], args[1]
	if formatVec.Type().ID() != super.IDString {
		return vector.NewWrappedError(s.zctx, "strftime: string value required for format arg", formatVec)
	}
	if timeVec.Type().ID() != super.IDTime {
		return vector.NewWrappedError(s.zctx, "strftime: time value required for time arg", args[1])
	}
	if cnst, ok := formatVec.(*vector.Const); ok {
		return s.fastPath(cnst, timeVec)
	}
	return s.slowPath(formatVec, timeVec)
}

func (s *Strftime) fastPath(fvec *vector.Const, tvec vector.Any) vector.Any {
	format, _ := fvec.AsString()
	f, err := strftime.New(format)
	if err != nil {
		return vector.NewWrappedError(s.zctx, "strftime: "+err.Error(), fvec)
	}
	switch tvec := tvec.(type) {
	case *vector.Int:
		return s.fastPathLoop(f, tvec, nil)
	case *vector.View:
		return s.fastPathLoop(f, tvec.Any.(*vector.Int), tvec.Index)
	case *vector.Dict:
		vec := s.fastPathLoop(f, tvec.Any.(*vector.Int), nil)
		return vector.NewDict(vec, tvec.Index, tvec.Counts, tvec.Nulls)
	case *vector.Const:
		t, _ := tvec.AsInt()
		s := f.FormatString(nano.Ts(t).Time())
		return vector.NewConst(super.NewString(s), tvec.Len(), tvec.Nulls)
	default:
		panic(tvec)
	}
}

func (s *Strftime) fastPathLoop(f *strftime.Strftime, vec *vector.Int, index []uint32) *vector.String {
	out := vector.NewStringEmpty(vec.Len(), vec.Nulls)
	for i := range vec.Len() {
		idx := i
		if index != nil {
			idx = index[i]
		}
		s := f.FormatString(nano.Ts(vec.Values[idx]).Time())
		out.Append(s)
	}
	return out
}

func (s *Strftime) slowPath(fvec vector.Any, tvec vector.Any) vector.Any {
	var f *strftime.Strftime
	var errIndex []uint32
	errMsgs := vector.NewStringEmpty(0, nil)
	out := vector.NewStringEmpty(0, vector.NewBoolEmpty(tvec.Len(), nil))
	for i := range fvec.Len() {
		format, _ := vector.StringValue(fvec, i)
		if f == nil || f.Pattern() != format {
			var err error
			f, err = strftime.New(format)
			if err != nil {
				errIndex = append(errIndex, i)
				errMsgs.Append("strftime: " + err.Error())
				continue
			}
		}
		t, isnull := vector.IntValue(tvec, i)
		if isnull {
			out.Nulls.Set(out.Len())
			out.Append("")
			continue
		}
		out.Append(f.FormatString(nano.Ts(t).Time()))
	}
	if len(errIndex) > 0 {
		errVec := vector.NewVecWrappedError(s.zctx, errMsgs, vector.NewView(errIndex, fvec))
		return vector.Combine(out, errIndex, errVec)
	}
	return out
}
