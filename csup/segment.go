package csup

import (
	"fmt"
	"io"
	"slices"
	"sync"

	"github.com/pierrec/lz4/v4"
)

// Values for [Segment.CompressionFormat].
const (
	CompressionFormatNone uint8 = 0 // No compression
	CompressionFormatLZ4  uint8 = 1 // LZ4 compression
)

type Segment struct {
	Offset            uint64 // Offset relative to start of file
	Length            uint64 // Length in file
	MemLength         uint64 // Length in memory
	CompressionFormat uint8  // Compression format in file
}

var zbufPool = sync.Pool{
	New: func() any { return new([]byte) },
}

// Read reads the segement r, uncompresses it if necessary, and stores it in the
// first s.MemLength bytes of b. If the length of b is less than s.MemLength,
// Read returns [io.ErrShortBuffer].
func (s *Segment) Read(r io.ReaderAt, b []byte) error {
	if len(b) < int(s.MemLength) {
		return fmt.Errorf("csup: segment read: %w", io.ErrShortBuffer)
	}
	b = b[:s.MemLength]
	switch s.CompressionFormat {
	case CompressionFormatNone:
		_, err := r.ReadAt(b, int64(s.Offset))
		return err
	case CompressionFormatLZ4:
		zbuf := zbufPool.Get().(*[]byte)
		defer zbufPool.Put(zbuf)
		*zbuf = slices.Grow((*zbuf)[:0], int(s.Length))[:s.Length]
		if _, err := r.ReadAt(*zbuf, int64(s.Offset)); err != nil {
			return err
		}
		n, err := lz4.UncompressBlock(*zbuf, b)
		if err != nil {
			return err
		}
		if n != int(s.MemLength) {
			return fmt.Errorf("csup: got %d uncompressed bytes, expected %d", n, s.MemLength)
		}
		return nil
	default:
		return fmt.Errorf("csup: unknown compression format 0x%x", s.CompressionFormat)
	}
}

// XXX for now we always compress, we should add a config option to
// avoid compression when local storage is fast compared to compute
func compressBuffer(b []byte) (uint8, []byte, error) {
	inLen := len(b)
	if inLen == 0 {
		return CompressionFormatNone, nil, nil
	}
	zbuf := zbufPool.Get().(*[]byte)
	defer zbufPool.Put(zbuf)
	// Use inLen-1 so compression will fail if it doesn't result in
	// fewer bytes. XXX make the -1 a bigger gap
	*zbuf = slices.Grow((*zbuf)[:0], inLen-1)[:inLen-1]
	var c lz4.Compressor
	zlen, err := c.CompressBlock(b, *zbuf)
	if err != nil && err != lz4.ErrInvalidSourceShortBuffer {
		return 0, nil, err
	}
	if zlen > 0 {
		// Compression succeeded.  Copy bytes... XXX we should
		// have a way to stash the buffer in the Primitive and
		// release it after written.
		bytes := make([]byte, zlen)
		copy(bytes, *zbuf)
		return CompressionFormatLZ4, bytes, nil
	}
	return CompressionFormatNone, b, nil
}
