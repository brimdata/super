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
