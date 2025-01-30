package function

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/pkg/nano"
	"github.com/brimdata/super/runtime/vam/expr/cast"
	"github.com/brimdata/super/vector"
	"github.com/lestrrat-go/strftime"
)

// https://github.com/brimdata/super/blob/main/docs/language/functions.md#bucket
type Bucket struct {
	name string
	zctx *super.Context
}

func (b *Bucket) Call(args ...vector.Any) vector.Any {
	args = underAll(args)
	tsArg, binArg := args[0], args[1]
	tsID, binID := tsArg.Type().ID(), binArg.Type().ID()
	if tsID != super.IDDuration && tsID != super.IDTime {
		return vector.NewWrappedError(b.zctx, b.name+": first argument is not a time or duration", tsArg)
	}
	if binID != super.IDDuration {
		return vector.NewWrappedError(b.zctx, b.name+": second argument is not a duration", binArg)
	}
	return vector.Apply(false, b.call, tsArg, binArg)
}

func (b *Bucket) call(args ...vector.Any) vector.Any {
	tsArg, binArg := args[0], args[1]
	if tsArg.Type().Kind() == super.ErrorKind {
		return tsArg
	}
	if binArg.Type().Kind() == super.ErrorKind {
		return binArg
	}
	if constBin, ok := binArg.(*vector.Const); ok {
		// Optimize case where the bin argument is static.
		bin, _ := constBin.AsInt()
		return b.constBin(tsArg, nano.Duration(bin))
	}
	var ints []int64
	for i := range tsArg.Len() {
		dur, _ := vector.IntValue(tsArg, i)
		bin, _ := vector.IntValue(binArg, i)
		if bin == 0 {
			ints = append(ints, dur)
		} else {
			ints = append(ints, int64(nano.Ts(dur).Trunc(nano.Duration(bin))))
		}
	}
	nulls := vector.Or(vector.NullsOf(tsArg), vector.NullsOf(binArg))
	return vector.NewInt(b.resultType(tsArg), ints, nulls)
}

func (b *Bucket) constBin(tsVec vector.Any, bin nano.Duration) vector.Any {
	if bin == 0 {
		return cast.To(b.zctx, tsVec, b.resultType(tsVec))
	}
	switch tsVec := tsVec.(type) {
	case *vector.Const:
		ts, _ := tsVec.AsInt()
		val := super.NewInt(b.resultType(tsVec), int64(nano.Ts(ts).Trunc(bin)))
		return vector.NewConst(val, tsVec.Len(), tsVec.Nulls)
	case *vector.View:
		return vector.NewView(b.constBinFlat(tsVec.Any, bin), tsVec.Index)
	case *vector.Dict:
		return vector.NewDict(b.constBinFlat(tsVec.Any, bin), tsVec.Index, tsVec.Counts, tsVec.Nulls)
	default:
		return b.constBinFlat(tsVec, bin)
	}
}

func (b *Bucket) constBinFlat(tsVecFlat vector.Any, bin nano.Duration) vector.Any {
	tsVec := tsVecFlat.(*vector.Int)
	ints := make([]int64, tsVec.Len())
	for i := range tsVec.Len() {
		if bin == 0 {
			ints[i] = tsVec.Values[i]
		} else {
			ints[i] = int64(nano.Ts(tsVec.Values[i]).Trunc(bin))
		}
	}
	return vector.NewInt(b.resultType(tsVec), ints, tsVec.Nulls)
}

func (b *Bucket) resultType(tsVec vector.Any) super.Type {
	if tsVec.Type().ID() == super.IDDuration {
		return super.TypeDuration
	}
	return super.TypeTime
}

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
	if index != nil {
		out := vector.NewStringEmpty(uint32(len(index)), vector.NewBoolView(vec.Nulls, index))
		for _, i := range index {
			s := f.FormatString(nano.Ts(vec.Values[i]).Time())
			out.Append(s)
		}
		return out
	}
	out := vector.NewStringEmpty(vec.Len(), vec.Nulls)
	for i := range vec.Len() {
		s := f.FormatString(nano.Ts(vec.Values[i]).Time())
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
		errVec := vector.NewVecWrappedError(s.zctx, errMsgs, vector.NewView(fvec, errIndex))
		return vector.Combine(out, errIndex, errVec)
	}
	return out
}
