package vector

type Labeled struct {
	Any
	Label string
}

func Unlabel(vec Any) (Any, string) {
	if vec, ok := vec.(*Labeled); ok {
		return vec.Any, vec.Label
	}
	return vec, ""
}

func Strip(vec Any) Any {
	switch vec := vec.(type) {
	case *Control:
		return vec.Any
	case *Labeled:
		return vec.Any
	default:
		return vec
	}
}

type Control struct {
	Any
}
