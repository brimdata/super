package parser

import (
	"fmt"
	"os"
	"strings"

	"github.com/brimdata/super/compiler/ast"
)

// ParseSuperPipe calls ConcatSource followed by Parse.  If Parse returns an error,
// ConcatSource tries to convert it to an ErrorList.
func ParseSuperPipe(sql bool, filenames []string, src string) (ast.Seq, *SourceSet, error) {
	sset, err := ConcatSource(filenames, src)
	if err != nil {
		return nil, nil, err
	}
	var opt Option
	if sql {
		opt = Entrypoint("SuperSQL")
	} else {
		opt = Entrypoint("start")
	}
	p, err := Parse("", []byte(sset.Text), opt, Recover(false))
	if err != nil {
		return nil, nil, convertErrList(err, sset)
	}
	return sliceOf[ast.Op](p), sset, nil
}

// ConcatSource concatenates the source files in filenames followed by src,
// returning a SourceSet.
func ConcatSource(filenames []string, src string) (*SourceSet, error) {
	var b strings.Builder
	var sis []*SourceInfo
	for _, f := range filenames {
		bb, err := os.ReadFile(f)
		if err != nil {
			return nil, err
		}
		sis = append(sis, newSourceInfo(f, b.Len(), bb))
		b.Write(bb)
		b.WriteByte('\n')
	}
	if b.Len() == 0 && src == "" {
		src = "*"
	}
	sis = append(sis, newSourceInfo("", b.Len(), []byte(src)))
	b.WriteString(src)
	return &SourceSet{b.String(), sis}, nil
}

func convertErrList(err error, sset *SourceSet) error {
	errs, ok := err.(errList)
	if !ok {
		return err
	}
	var out ErrorList
	for _, e := range errs {
		pe, ok := e.(*parserError)
		if !ok {
			return err
		}
		out.Append("error parsing SuperPipe", pe.pos.offset, -1)
	}
	out.SetSourceSet(sset)
	return out
}

// ErrList is a list of Errors.
type ErrorList []*Error

// Append appends an Error to e.
func (e *ErrorList) Append(msg string, pos, end int) {
	*e = append(*e, &Error{msg, pos, end, nil})
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

// SetSourceSet sets the SourceSet for every Error in e.
func (e ErrorList) SetSourceSet(sset *SourceSet) {
	for i := range e {
		e[i].sset = sset
	}
}

type Error struct {
	Msg  string
	Pos  int
	End  int
	sset *SourceSet
}

func (e *Error) Error() string {
	if e.sset == nil {
		return e.Msg
	}
	src := e.sset.SourceOf(e.Pos)
	start := src.Position(e.Pos)
	end := src.Position(e.End)
	var b strings.Builder
	b.WriteString(e.Msg)
	if src.Filename != "" {
		fmt.Fprintf(&b, " in %s", src.Filename)
	}
	line := src.LineOfPos(e.sset.Text, e.Pos)
	fmt.Fprintf(&b, " at line %d, column %d:\n%s\n", start.Line, start.Column, line)
	if end.IsValid() {
		formatSpanError(&b, line, start, end)
	} else {
		formatPointError(&b, start)
	}
	return b.String()
}

func formatSpanError(b *strings.Builder, line string, start, end Position) {
	col := start.Column - 1
	b.WriteString(strings.Repeat(" ", col))
	n := len(line) - col
	if start.Line == end.Line {
		n = end.Column - 1 - col
	}
	b.WriteString(strings.Repeat("~", n))
}

func formatPointError(b *strings.Builder, start Position) {
	col := start.Column - 1
	for k := 0; k < col; k++ {
		if k >= col-4 && k != col-1 {
			b.WriteByte('=')
		} else {
			b.WriteByte(' ')
		}
	}
	b.WriteString("^ ===")
}
