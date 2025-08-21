package bsupio

import (
	"fmt"

	"github.com/pierrec/lz4/v4"
)

const (
	TypesFrame   = 0
	ValuesFrame  = 1
	ControlFrame = 2
)

const (
	EOS                 = 0xff
	ControlFormatBSUP   = 0
	ControlFormatJSON   = 1
	ControlFormatSUP    = 2
	ControlFormatString = 3
	ControlFormatBinary = 4
)

type CompressionFormat int

const CompressionFormatLZ4 CompressionFormat = 0x00

type frame struct {
	fmt  CompressionFormat
	sbuf *buffer
	ubuf *buffer
}

func (f *frame) free() {
	f.sbuf.free()
	f.ubuf.free()
}

func (f *frame) decompress() error {
	if f.fmt != CompressionFormatLZ4 {
		return fmt.Errorf("bsupio: unknown compression format 0x%x", f.fmt)
	}
	n, err := lz4.UncompressBlock(f.sbuf.data, f.ubuf.data)
	if err != nil {
		return fmt.Errorf("bsupio: %w", err)
	}
	if n != len(f.ubuf.data) {
		return fmt.Errorf("bsupio: got %d uncompressed bytes, expected %d", n, len(f.ubuf.data))
	}
	return nil
}
