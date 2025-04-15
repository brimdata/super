package srcfiles

import (
	"fmt"
	"strings"
)

// ErrList is a list of Errors.
type ErrorList []*Error

// Append appends an Error to e.
func (e *ErrorList) Append(list *List, msg string, pos, end int) {
	*e = append(*e, &Error{msg, pos, end, list})
}

// Bind takes errors that were created elsewhere (e.g., the service) using
// the list's files and points the errors back at this list.
func (e ErrorList) Bind(list *List) {
	for i := range e {
		e[i].list = list
	}
}

// Error concatenates the errors in e with a newline between each.
func (e ErrorList) Error() string {
	var b strings.Builder
	for i, err := range e {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(err.Error())
	}
	return b.String()
}

type Error struct {
	Msg  string
	Pos  int
	End  int
	list *List
}

func (e *Error) Error() string {
	if e.list == nil {
		return e.Msg
	}
	file := e.list.FileOf(e.Pos)
	start := file.Position(e.Pos)
	end := file.Position(e.End)
	var b strings.Builder
	b.WriteString(e.Msg)
	if file.Name != "" {
		fmt.Fprintf(&b, " in %s", file.Name)
	}
	line := file.LineOfPos(e.list.Text, e.Pos)
	fmt.Fprintf(&b, " at line %d, column %d:\n%s\n", start.Line, start.Column, line)
	if end.IsValid() {
		formatSpanError(&b, line, start, end)
	} else {
		formatPointError(&b, start)
	}
	return b.String()
}

func formatSpanError(b *strings.Builder, line string, start, end Position) {
	b.WriteString(strings.Repeat(" ", start.Column-1))
	n := end.Column - start.Column + 1
	if start.Line != end.Line {
		n = len(line) - start.Column + 1
	}
	b.WriteString(strings.Repeat("~", n))
}

func formatPointError(b *strings.Builder, start Position) {
	col := start.Column - 1
	for k := range col {
		if k >= col-4 && k != col-1 {
			b.WriteByte('=')
		} else {
			b.WriteByte(' ')
		}
	}
	b.WriteString("^ ===")
}
