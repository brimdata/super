// Code generated by gendatepart.go. DO NOT EDIT.

package function

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/pkg/nano"
	"github.com/brimdata/super/vector"
)

func date_time_dayofweek(vec vector.Any) vector.Any {
	switch vec := vec.(type) {
	case *vector.View:
		index := vec.Index
		inner := vec.Any.(*vector.Int)
		out := make([]int64, len(index))
		for i, idx := range index {
			v := inner.Values[idx]
			out[i] = int64(nano.Ts(v).Time().Weekday())
		}
		return vector.NewInt(super.TypeInt64, out, inner.Nulls.Pick(index))
	case *vector.Const:
		v := vec.Value().Int()
		val := super.NewInt64(int64(nano.Ts(v).Time().Weekday()))
		return vector.NewConst(val, vec.Len(), vec.Nulls)
	case *vector.Dict:
		out := date_time_dayofweek(vec.Any).(*vector.Int)
		return vector.NewDict(out, vec.Index, vec.Counts, vec.Nulls)
	case *vector.Int:
		out := make([]int64, vec.Len())
		for i, v := range vec.Values {
			out[i] = int64(nano.Ts(v).Time().Weekday())
		}
		return vector.NewInt(super.TypeInt64, out, vec.Nulls)
	default:
		panic(vec)
	}
}

func date_time_day(vec vector.Any) vector.Any {
	switch vec := vec.(type) {
	case *vector.View:
		index := vec.Index
		inner := vec.Any.(*vector.Int)
		out := make([]int64, len(index))
		for i, idx := range index {
			v := inner.Values[idx]
			out[i] = int64(nano.Ts(v).Time().Day())
		}
		return vector.NewInt(super.TypeInt64, out, inner.Nulls.Pick(index))
	case *vector.Const:
		v := vec.Value().Int()
		val := super.NewInt64(int64(nano.Ts(v).Time().Day()))
		return vector.NewConst(val, vec.Len(), vec.Nulls)
	case *vector.Dict:
		out := date_time_day(vec.Any).(*vector.Int)
		return vector.NewDict(out, vec.Index, vec.Counts, vec.Nulls)
	case *vector.Int:
		out := make([]int64, vec.Len())
		for i, v := range vec.Values {
			out[i] = int64(nano.Ts(v).Time().Day())
		}
		return vector.NewInt(super.TypeInt64, out, vec.Nulls)
	default:
		panic(vec)
	}
}

func date_time_dow(vec vector.Any) vector.Any {
	switch vec := vec.(type) {
	case *vector.View:
		index := vec.Index
		inner := vec.Any.(*vector.Int)
		out := make([]int64, len(index))
		for i, idx := range index {
			v := inner.Values[idx]
			out[i] = int64(nano.Ts(v).Time().Weekday())
		}
		return vector.NewInt(super.TypeInt64, out, inner.Nulls.Pick(index))
	case *vector.Const:
		v := vec.Value().Int()
		val := super.NewInt64(int64(nano.Ts(v).Time().Weekday()))
		return vector.NewConst(val, vec.Len(), vec.Nulls)
	case *vector.Dict:
		out := date_time_dow(vec.Any).(*vector.Int)
		return vector.NewDict(out, vec.Index, vec.Counts, vec.Nulls)
	case *vector.Int:
		out := make([]int64, vec.Len())
		for i, v := range vec.Values {
			out[i] = int64(nano.Ts(v).Time().Weekday())
		}
		return vector.NewInt(super.TypeInt64, out, vec.Nulls)
	default:
		panic(vec)
	}
}

func date_time_hour(vec vector.Any) vector.Any {
	switch vec := vec.(type) {
	case *vector.View:
		index := vec.Index
		inner := vec.Any.(*vector.Int)
		out := make([]int64, len(index))
		for i, idx := range index {
			v := inner.Values[idx]
			out[i] = int64(nano.Ts(v).Time().Hour())
		}
		return vector.NewInt(super.TypeInt64, out, inner.Nulls.Pick(index))
	case *vector.Const:
		v := vec.Value().Int()
		val := super.NewInt64(int64(nano.Ts(v).Time().Hour()))
		return vector.NewConst(val, vec.Len(), vec.Nulls)
	case *vector.Dict:
		out := date_time_hour(vec.Any).(*vector.Int)
		return vector.NewDict(out, vec.Index, vec.Counts, vec.Nulls)
	case *vector.Int:
		out := make([]int64, vec.Len())
		for i, v := range vec.Values {
			out[i] = int64(nano.Ts(v).Time().Hour())
		}
		return vector.NewInt(super.TypeInt64, out, vec.Nulls)
	default:
		panic(vec)
	}
}

func date_time_microseconds(vec vector.Any) vector.Any {
	switch vec := vec.(type) {
	case *vector.View:
		index := vec.Index
		inner := vec.Any.(*vector.Int)
		out := make([]int64, len(index))
		for i, idx := range index {
			v := inner.Values[idx]
			out[i] = int64(nano.Ts(v).Time().Second()*1e6 + nano.Ts(v).Time().Nanosecond()/1e3)
		}
		return vector.NewInt(super.TypeInt64, out, inner.Nulls.Pick(index))
	case *vector.Const:
		v := vec.Value().Int()
		val := super.NewInt64(int64(nano.Ts(v).Time().Second()*1e6 + nano.Ts(v).Time().Nanosecond()/1e3))
		return vector.NewConst(val, vec.Len(), vec.Nulls)
	case *vector.Dict:
		out := date_time_microseconds(vec.Any).(*vector.Int)
		return vector.NewDict(out, vec.Index, vec.Counts, vec.Nulls)
	case *vector.Int:
		out := make([]int64, vec.Len())
		for i, v := range vec.Values {
			out[i] = int64(nano.Ts(v).Time().Second()*1e6 + nano.Ts(v).Time().Nanosecond()/1e3)
		}
		return vector.NewInt(super.TypeInt64, out, vec.Nulls)
	default:
		panic(vec)
	}
}

func date_time_milliseconds(vec vector.Any) vector.Any {
	switch vec := vec.(type) {
	case *vector.View:
		index := vec.Index
		inner := vec.Any.(*vector.Int)
		out := make([]int64, len(index))
		for i, idx := range index {
			v := inner.Values[idx]
			out[i] = int64(nano.Ts(v).Time().Second()*1e3 + nano.Ts(v).Time().Nanosecond()/1e6)
		}
		return vector.NewInt(super.TypeInt64, out, inner.Nulls.Pick(index))
	case *vector.Const:
		v := vec.Value().Int()
		val := super.NewInt64(int64(nano.Ts(v).Time().Second()*1e3 + nano.Ts(v).Time().Nanosecond()/1e6))
		return vector.NewConst(val, vec.Len(), vec.Nulls)
	case *vector.Dict:
		out := date_time_milliseconds(vec.Any).(*vector.Int)
		return vector.NewDict(out, vec.Index, vec.Counts, vec.Nulls)
	case *vector.Int:
		out := make([]int64, vec.Len())
		for i, v := range vec.Values {
			out[i] = int64(nano.Ts(v).Time().Second()*1e3 + nano.Ts(v).Time().Nanosecond()/1e6)
		}
		return vector.NewInt(super.TypeInt64, out, vec.Nulls)
	default:
		panic(vec)
	}
}

func date_time_minute(vec vector.Any) vector.Any {
	switch vec := vec.(type) {
	case *vector.View:
		index := vec.Index
		inner := vec.Any.(*vector.Int)
		out := make([]int64, len(index))
		for i, idx := range index {
			v := inner.Values[idx]
			out[i] = int64(nano.Ts(v).Time().Minute())
		}
		return vector.NewInt(super.TypeInt64, out, inner.Nulls.Pick(index))
	case *vector.Const:
		v := vec.Value().Int()
		val := super.NewInt64(int64(nano.Ts(v).Time().Minute()))
		return vector.NewConst(val, vec.Len(), vec.Nulls)
	case *vector.Dict:
		out := date_time_minute(vec.Any).(*vector.Int)
		return vector.NewDict(out, vec.Index, vec.Counts, vec.Nulls)
	case *vector.Int:
		out := make([]int64, vec.Len())
		for i, v := range vec.Values {
			out[i] = int64(nano.Ts(v).Time().Minute())
		}
		return vector.NewInt(super.TypeInt64, out, vec.Nulls)
	default:
		panic(vec)
	}
}

func date_time_month(vec vector.Any) vector.Any {
	switch vec := vec.(type) {
	case *vector.View:
		index := vec.Index
		inner := vec.Any.(*vector.Int)
		out := make([]int64, len(index))
		for i, idx := range index {
			v := inner.Values[idx]
			out[i] = int64(nano.Ts(v).Time().Month())
		}
		return vector.NewInt(super.TypeInt64, out, inner.Nulls.Pick(index))
	case *vector.Const:
		v := vec.Value().Int()
		val := super.NewInt64(int64(nano.Ts(v).Time().Month()))
		return vector.NewConst(val, vec.Len(), vec.Nulls)
	case *vector.Dict:
		out := date_time_month(vec.Any).(*vector.Int)
		return vector.NewDict(out, vec.Index, vec.Counts, vec.Nulls)
	case *vector.Int:
		out := make([]int64, vec.Len())
		for i, v := range vec.Values {
			out[i] = int64(nano.Ts(v).Time().Month())
		}
		return vector.NewInt(super.TypeInt64, out, vec.Nulls)
	default:
		panic(vec)
	}
}

func date_time_second(vec vector.Any) vector.Any {
	switch vec := vec.(type) {
	case *vector.View:
		index := vec.Index
		inner := vec.Any.(*vector.Int)
		out := make([]int64, len(index))
		for i, idx := range index {
			v := inner.Values[idx]
			out[i] = int64(nano.Ts(v).Time().Second())
		}
		return vector.NewInt(super.TypeInt64, out, inner.Nulls.Pick(index))
	case *vector.Const:
		v := vec.Value().Int()
		val := super.NewInt64(int64(nano.Ts(v).Time().Second()))
		return vector.NewConst(val, vec.Len(), vec.Nulls)
	case *vector.Dict:
		out := date_time_second(vec.Any).(*vector.Int)
		return vector.NewDict(out, vec.Index, vec.Counts, vec.Nulls)
	case *vector.Int:
		out := make([]int64, vec.Len())
		for i, v := range vec.Values {
			out[i] = int64(nano.Ts(v).Time().Second())
		}
		return vector.NewInt(super.TypeInt64, out, vec.Nulls)
	default:
		panic(vec)
	}
}

func date_time_year(vec vector.Any) vector.Any {
	switch vec := vec.(type) {
	case *vector.View:
		index := vec.Index
		inner := vec.Any.(*vector.Int)
		out := make([]int64, len(index))
		for i, idx := range index {
			v := inner.Values[idx]
			out[i] = int64(nano.Ts(v).Time().Year())
		}
		return vector.NewInt(super.TypeInt64, out, inner.Nulls.Pick(index))
	case *vector.Const:
		v := vec.Value().Int()
		val := super.NewInt64(int64(nano.Ts(v).Time().Year()))
		return vector.NewConst(val, vec.Len(), vec.Nulls)
	case *vector.Dict:
		out := date_time_year(vec.Any).(*vector.Int)
		return vector.NewDict(out, vec.Index, vec.Counts, vec.Nulls)
	case *vector.Int:
		out := make([]int64, vec.Len())
		for i, v := range vec.Values {
			out[i] = int64(nano.Ts(v).Time().Year())
		}
		return vector.NewInt(super.TypeInt64, out, vec.Nulls)
	default:
		panic(vec)
	}
}

var datePartFuncs = map[string]func(vector.Any) vector.Any{
	"dayofweek":    date_time_dayofweek,
	"day":          date_time_day,
	"dow":          date_time_dow,
	"hour":         date_time_hour,
	"microseconds": date_time_microseconds,
	"milliseconds": date_time_milliseconds,
	"minute":       date_time_minute,
	"month":        date_time_month,
	"second":       date_time_second,
	"year":         date_time_year,
}
