package zeekio

import (
	"bufio"
	"errors"
	"fmt"
	"io"

	"github.com/brimdata/super"
)

const (
	ReadSize    = 64 * 1024
	MaxLineSize = 50 * 1024 * 1024
)

type Reader struct {
	scanner *bufio.Scanner
	parser  *Parser
	lines   int
}

func NewReader(sctx *super.Context, reader io.Reader) *Reader {
	s := bufio.NewScanner(reader)
	s.Buffer(make([]byte, ReadSize), MaxLineSize)
	return &Reader{
		scanner: s,
		parser:  NewParser(sctx),
	}
}

func (r *Reader) Read() (*super.Value, error) {
	e := func(err error) error {
		if err == nil {
			return err
		}
		if errors.Is(err, bufio.ErrTooLong) {
			err = errors.New("line too long")
		}
		return fmt.Errorf("line %d: %w", r.lines, err)
	}
	for {
		r.lines++
		if !r.scanner.Scan() {
			return nil, e(r.scanner.Err())
		}
		line := r.scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		if line[0] == '#' {
			if err := r.parser.ParseDirective(line); err != nil {
				return nil, e(err)
			}
			continue
		}
		val, err := r.parser.ParseValue(line)
		return val, e(err)
	}

}
