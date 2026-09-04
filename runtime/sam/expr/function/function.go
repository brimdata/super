package function

import (
	"errors"

	"github.com/brimdata/super"
)

var (
	ErrNoSuchFunction = errors.New("no such function")
	ErrTooFewArgs     = errors.New("too few arguments")
	ErrTooManyArgs    = errors.New("too many arguments")
)

func underAll(args []super.Value) []super.Value {
	for i := range args {
		args[i] = args[i].Under()
	}
	return args
}
