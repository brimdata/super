package vector

import (
	"io"

	"github.com/brimdata/zed/zcode"
)

// NullsWriter emits a sequence of runs of the length of alternating sequences
// of nulls and values, beginning with nulls.  Every run is non-zero except for
// the first, which may be zero when the first value is non-null.
type NullsWriter struct {
	values Writer
	runs   Int64Writer
	run    int64
	null   bool
	dirty  bool
}

func NewNullsWriter(values Writer, spiller *Spiller) *NullsWriter {
	return &NullsWriter{
		values: values,
		runs:   *NewInt64Writer(spiller),
	}
}

func (n *NullsWriter) Write(body zcode.Bytes) error {
	if body != nil {
		n.touchValue()
		return n.values.Write(body)
	}
	n.touchNull()
	return nil
}

func (n *NullsWriter) touchValue() {
	if !n.null {
		n.run++
	} else {
		n.runs.Write(n.run)
		n.run = 1
		n.null = false
	}
}

func (n *NullsWriter) touchNull() {
	n.dirty = true
	if n.null {
		n.run++
	} else {
		n.runs.Write(n.run)
		n.run = 1
		n.null = true
	}
}

func (n *NullsWriter) Flush(eof bool) error {
	if eof && n.dirty {
		if err := n.runs.Write(n.run); err != nil {
			return err
		}
		if err := n.runs.Flush(true); err != nil {
			return err
		}
	}
	return n.values.Flush(eof)
}

func (n *NullsWriter) Metadata() Metadata {
	values := n.values.Metadata()
	runs := n.runs.segments
	if len(runs) == 0 {
		return values
	}
	return &Nulls{
		Runs:   runs,
		Values: values,
	}
}

type NullsReader struct {
	Values Reader
	Runs   Int64Reader
	null   bool
	run    int
}

func NewNullsReader(values Reader, segmap []Segment, r io.ReaderAt) *NullsReader {
	// We start out with null true so it is immediately flipped to
	// false on the first call to Read.
	return &NullsReader{
		Values: values,
		Runs:   *NewInt64Reader(segmap, r),
		null:   true,
	}
}

func (n *NullsReader) Read(b *zcode.Builder) error {
	run := n.run
	for run == 0 {
		n.null = !n.null
		v, err := n.Runs.Read()
		if err != nil {
			return err
		}
		run = int(v)
	}
	n.run = run - 1
	if n.null {
		b.Append(nil)
		return nil
	}
	return n.Values.Read(b)
}
