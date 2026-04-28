package fjsonio

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"slices"

	"github.com/brimdata/super/pkg/jsonskip"
)

const maxSize = 512 * 1024 * 1024

type valReader struct {
	r      io.Reader
	buf    []byte
	cursor []byte
	EOF    bool
	line   int
}

func newValReader(r io.Reader) *valReader {
	return &valReader{r: r, buf: make([]byte, 512*1024)}
}

func (r *valReader) Next() ([]byte, error) {
	var hasFilled bool
	for {
		if r.EOF && len(r.cursor) == 0 {
			return nil, io.EOF
		}
		start, end := jsonskip.Skip(r.cursor)
		if start < 0 {
			if !hasFilled && !r.EOF {
				if err := r.fill(); err != nil {
					return nil, err
				}
				hasFilled = true
				continue
			}
			if hasFilled && !r.EOF {
				// Check if our buffer is at max size or not. It's unfortunate
				// to do this on every mid-stream parser error but we need to be
				// sure we're not failing because we've encountered a JSON value
				// that is too large to fit in buffer.
				if len(r.cursor) == len(r.buf) && len(r.buf) < maxSize {
					r.buf = slices.Grow(r.buf, maxSize)[:maxSize]
					continue
				}
			}
			if r.EOF && len(r.cursor) > 0 {
				// If we're at EOF but still have data in cursor make sure its
				// not just whitespace at the end of the file.
				if len(bytes.TrimSpace(r.cursor)) == 0 {
					return nil, io.EOF
				}
			}
			return nil, r.parseError()
		}
		b := r.cursor[start:end]
		r.line += bytes.Count(b, []byte{'\n'})
		r.cursor = r.cursor[end:]
		return b, nil
	}
}

func (r *valReader) fill() error {
	// copy rest of cursor to buf
	cc := copy(r.buf, r.cursor)
	n, err := r.r.Read(r.buf[cc:])
	if errors.Is(err, io.EOF) {
		r.EOF = true
		err = nil
	}
	if err != nil {
		return err
	}
	r.cursor = r.buf
	r.cursor = r.cursor[:cc+n]
	return nil
}

func (r *valReader) parseError() error {
	// The first character of a new line is often a newline which throws off
	// line count, so count newlines before first non-whitespace character.
	i := bytes.IndexFunc(r.cursor, func(r rune) bool {
		return r != ' ' && r != '\t' && r != '\n' && r != '\r'
	})
	if i != -1 {
		r.line += bytes.Count(r.cursor[:i], []byte{'\n'})
	}
	return fmt.Errorf("line %d: invalid JSON value", r.line+1)
}
