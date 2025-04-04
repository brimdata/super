package seekindex

import (
	"fmt"
	"io"
)

type Range struct {
	Offset int64 `super:"offset"`
	Length int64 `super:"length"`
}

func (r Range) IsZero() bool {
	return r.Length == 0
}

func (r Range) String() string {
	return fmt.Sprintf("offset %d length %d", r.Offset, r.Length)
}

func (r Range) Reader(reader io.ReaderAt) (io.Reader, error) {
	return io.NewSectionReader(reader, r.Offset, r.Length), nil
}

type Ranges []Range

func (r *Ranges) Append(entry Entry) {
	n := len(*r)
	if n == 0 || (*r)[n-1].Offset+(*r)[n-1].Length < int64(entry.Offset) {
		*r = append(*r, entry.Range())
		return
	}
	(*r)[n-1].Length += int64(entry.Length)
}
