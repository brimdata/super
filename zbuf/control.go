package zbuf

type Control struct {
	Message any
}

var _ error = (*Control)(nil)

func (c *Control) Error() string {
	return "control"
}
