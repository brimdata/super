package zeekio

import (
	"bytes"
	"fmt"
	"io"

	"github.com/brimdata/zed"
	"github.com/brimdata/zed/pkg/skim"
)

const (
	ReadSize    = 64 * 1024
	MaxLineSize = 50 * 1024 * 1024
)

type Reader struct {
	arena   *zed.Arena
	scanner *skim.Scanner
	parser  *Parser
}

func NewReader(zctx *zed.Context, reader io.Reader) *Reader {
	buffer := make([]byte, ReadSize)
	return &Reader{
		arena:   zed.NewArena(),
		scanner: skim.NewScanner(reader, buffer, MaxLineSize),
		parser:  NewParser(zctx),
	}
}

func (r *Reader) Read() (*zed.Value, error) {
	e := func(err error) error {
		if err == nil {
			return err
		}
		return fmt.Errorf("line %d: %w", r.scanner.Stats.Lines, err)
	}

again:
	line, err := r.scanner.ScanLine()
	if line == nil {
		if err != nil {
			return nil, e(err)
		}
		return nil, nil
	}
	// remove newline
	line = bytes.TrimSuffix(line, []byte("\n"))
	if line[0] == '#' {

		if err := r.parser.ParseDirective(line); err != nil {
			return nil, e(err)
		}
		goto again
	}
	r.arena.Reset()
	rec, err := r.parser.ParseValue(r.arena, line)
	if err != nil {
		return nil, e(err)
	}
	return rec, nil
}
