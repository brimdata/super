// Package srverr provides a mechanism to create or wrap errors with information
// that will aid in reporting them to users and returning them to api callers.
package srverr

import (
	"errors"
	"fmt"
	"runtime"
	"runtime/debug"
	"strings"
)

// A Kind represents a class of error. API layers will typically convert
// these into a domain specific error representation; for example, an HTTP
// handler can convert these to HTTP status codes.
type Kind int

const (
	Other Kind = iota
	Conflict
	Exists
	Forbidden
	Invalid
	NoCredentials
	NotFound
)

func (k Kind) String() string {
	switch k {
	case Conflict:
		return "conflict with pending operation"
	case Exists:
		return "item already exists"
	case Invalid:
		return "invalid operation"
	case Forbidden:
		return "forbidden"
	case NoCredentials:
		return "missing authentication credentials"
	case NotFound:
		return "item does not exist"
	case Other:
		return "other error"
	}
	return "unknown error kind"
}

type Error struct {
	Kind Kind
	Err  error
}

func pad(b *strings.Builder, s string) {
	if b.Len() == 0 {
		return
	}
	b.WriteString(s)
}

func (e *Error) Error() string {
	b := strings.Builder{}
	if e.Kind != Other {
		b.WriteString(e.Kind.String())
	}
	if e.Err != nil {
		pad(&b, ": ")
		b.WriteString(e.Err.Error())
	}
	if b.Len() == 0 {
		return "no error"
	}
	return b.String()
}

func (e *Error) Unwrap() error {
	return e.Err
}

func (e *Error) Is(target error) bool {
	if zt, ok := target.(*Error); ok {
		return zt.Kind == e.Kind
	}
	return false
}

// Message returns just the Err.Error() string, if present, or the Kind
// string description. The intent is to allow srverr users a way to avoid
// embedding the Kind description as happens with Error().
func (e *Error) Message() string {
	if e.Err != nil {
		return e.Err.Error()
	}
	if e.Kind != Other {
		return e.Kind.String()
	}
	return "no error"
}

// Function E generates an error from any mix of:
//   - a Kind
//   - an existing error
//   - a string and optional formatting verbs, like fmt.Errorf (including support
//     for the `%w` verb).
//
// The string & format verbs must be last in the arguments, if present.
func E(args ...interface{}) error {
	if len(args) == 0 {
		panic("no args to errors.E")
	}
	e := &Error{}

	for i, arg := range args {
		switch arg := arg.(type) {
		case Kind:
			e.Kind = arg
		case error:
			e.Err = arg
		case string:
			if i == len(args)-1 {
				e.Err = errors.New(arg)
				return e
			}
			e.Err = fmt.Errorf(arg, args[i+1:]...)
			return e
		default:
			_, file, line, _ := runtime.Caller(1)
			return fmt.Errorf("unknown type %T value %v in srverr.E call at %v:%v", arg, arg, file, line)
		}
	}

	return e
}

// IsKind returns true if the provided error can be unwrapped as a *Error and if
// *Error.Kind matches the provided Kind
func IsKind(err error, k Kind) bool {
	var zerr *Error
	return errors.As(err, &zerr) && zerr.Kind == k
}

func IsConflict(err error) bool      { return IsKind(err, Conflict) }
func IsExists(err error) bool        { return IsKind(err, Exists) }
func IsForbidden(err error) bool     { return IsKind(err, Forbidden) }
func IsInvalid(err error) bool       { return IsKind(err, Invalid) }
func IsNoCredentials(err error) bool { return IsKind(err, NoCredentials) }
func IsNotFound(err error) bool      { return IsKind(err, NotFound) }
func IsOther(err error) bool         { return IsKind(err, Other) }

func ErrConflict(args ...interface{}) error      { return errKind(Conflict, args) }
func ErrExists(args ...interface{}) error        { return errKind(Exists, args) }
func ErrForbidden(args ...interface{}) error     { return errKind(Forbidden, args) }
func ErrInvalid(args ...interface{}) error       { return errKind(Invalid, args) }
func ErrNoCredentials(args ...interface{}) error { return errKind(NoCredentials, args) }
func ErrNotFound(args ...interface{}) error      { return errKind(NotFound, args) }
func ErrOther(args ...interface{}) error         { return errKind(Other, args) }

func errKind(k Kind, args []interface{}) error {
	args = append([]interface{}{k}, args...)
	return E(args...)
}

func RecoverError(r interface{}) error {
	return E("panic: %+v\n%s\n", r, string(debug.Stack()))
}
