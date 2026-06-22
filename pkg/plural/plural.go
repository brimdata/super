package plural

// Slice returns an empty string if the slice has exactly one element, otherwise returns the given suffix string. This is useful for handling pluralization in output strings.
func Slice[S ~[]E, E any](s S, suffix string) string {
	if len(s) == 1 {
		return ""
	}
	return suffix
}
