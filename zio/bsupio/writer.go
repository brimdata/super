package bsupio

import (
	"encoding/binary"
	"io"
	"slices"

	"github.com/brimdata/super"
	"github.com/brimdata/super/zcode"
	"github.com/pierrec/lz4/v4"
)

// DefaultFrameThresh is a reasonable default for WriterOpts.FrameThresh.
const DefaultFrameThresh = 512 * 1024

type Writer struct {
	writer     io.WriteCloser
	position   int64
	flushed    int64
	compressor *compressor
	opts       WriterOpts

	types  *Encoder
	values []byte
	header []byte
}

type WriterOpts struct {
	Compress bool
	// FrameThresh is the minimum frame size in uncompressed bytes.
	FrameThresh int
}

// NewWriter returns a writer to w with reasonable default options.
// Specifically, it enables compression and sets the frame threshold to
// DefaultFrameThresh.
func NewWriter(w io.WriteCloser) *Writer {
	return NewWriterWithOpts(w, WriterOpts{
		Compress:    true,
		FrameThresh: DefaultFrameThresh,
	})
}

// NewWriterWithOpts returns a writer to w with opts.
func NewWriterWithOpts(w io.WriteCloser, opts WriterOpts) *Writer {
	var comp *compressor
	if opts.Compress {
		comp = &compressor{}
	}
	return &Writer{
		writer:     w,
		compressor: comp,
		opts:       opts,
		types:      NewEncoder(),
	}
}

func (w *Writer) DisableCompression() {
	w.compressor = nil
}

func (w *Writer) Close() error {
	err := w.EndStream()
	if closeErr := w.writer.Close(); err == nil {
		err = closeErr
	}
	return err
}

func (w *Writer) write(p []byte) error {
	n, err := w.writer.Write(p)
	if err != nil {
		return err
	}
	w.position += int64(n)
	return nil
}

// Position may be called after EndStream to get a seekable offset into the
// output for the next stream.  Calling Position at any other team returns
// unusable seek offsets.
func (w *Writer) Position() int64 {
	return w.position
}

func (w *Writer) EndStream() error {
	// Flush any compression state and write the EOS afterward the
	// compressed block since the buffer-filter may skip entire
	// compressed before and we would otherwise miss the EOS marker.
	if err := w.flush(); err != nil {
		return err
	}
	if w.flushed != w.position {
		if err := w.write([]byte{EOS}); err != nil {
			return err
		}
		w.flushed = w.position
	}
	w.types.Reset()
	return nil
}

func (w *Writer) Write(val super.Value) error {
	typ := w.types.Lookup(val.Type())
	if typ == nil {
		var err error
		typ, err = w.types.Encode(val.Type())
		if err != nil {
			return err
		}
	}
	id := super.TypeID(typ)
	w.values = binary.AppendUvarint(w.values, uint64(id))
	w.values = zcode.Append(w.values, val.Bytes())
	if thresh := w.opts.FrameThresh; len(w.values) >= thresh || len(w.types.bytes) >= thresh {
		return w.flush()
	}
	return nil
}

func (w *Writer) WriteControl(b []byte, format uint8) error {
	// Flush the compressor since we need to preserve the interleaving
	// order of control messages and BSON data.
	if err := w.flush(); err != nil {
		return err
	}
	// Yuck.
	bytes := make([]byte, len(b)+1)
	bytes[0] = format
	copy(bytes[1:], b)
	return w.writeBlock(ControlFrame, bytes)
}

func (w *Writer) flush() error {
	if err := w.writeBlock(TypesFrame, w.types.bytes); err != nil {
		return nil
	}
	if err := w.writeBlock(ValuesFrame, w.values); err != nil {
		return nil
	}
	w.types.Flush()
	w.values = w.values[:0]
	return nil
}

func (w *Writer) writeBlock(blockType int, b []byte) error {
	if len(b) == 0 {
		return nil
	}
	if w.compressor != nil {
		zbuf, err := w.compressor.compress(b)
		if err != nil {
			return err
		}
		if zbuf != nil {
			if err := w.writeCompHeader(blockType, len(b), len(zbuf)); err != nil {
				return err
			}
			return w.write(zbuf)
		}
	}
	if err := w.writeHeader(blockType, len(b)); err != nil {
		return err
	}
	return w.write(b)
}

func (w *Writer) writeHeader(blockType, size int) error {
	code := blockType<<4 | (size & 0xf)
	w.header = append(w.header[:0], byte(code))
	w.header = binary.AppendUvarint(w.header, uint64(size>>4))
	return w.write(w.header)
}

func (w *Writer) writeCompHeader(blockType, size, zlen int) error {
	zlen += 1 + zcode.SizeOfUvarint(uint64(size))
	code := (blockType << 4) | (zlen & 0xf) | 0x40
	w.header = append(w.header[:0], byte(code))
	w.header = binary.AppendUvarint(w.header, uint64(zlen>>4))
	w.header = append(w.header, byte(CompressionFormatLZ4))
	w.header = binary.AppendUvarint(w.header, uint64(size))
	return w.write(w.header)
}

type compressor struct {
	compressor lz4.Compressor
	zbuf       []byte
}

func (c *compressor) compress(b []byte) ([]byte, error) {
	if c == nil || len(b) == 0 {
		return nil, nil
	}
	c.zbuf = slices.Grow(c.zbuf[:0], len(b))
	zbuf := c.zbuf[:len(b)]
	zlen, err := c.compressor.CompressBlock(b, zbuf)
	if err != nil && err != lz4.ErrInvalidSourceShortBuffer {
		return nil, err
	}
	if zlen > 0 {
		// Compression succeeded and the compressed value message block
		// is smaller than the buffered messages, so write the
		// compressed value message block.
		return zbuf[:zlen], nil
	}
	return nil, nil
}
