package csup

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

const (
	Version     = 12
	HeaderSize  = 28
	MaxMetaSize = 100 * 1024 * 1024
	MaxDataSize = 2 * 1024 * 1024 * 1024
)

type Header struct {
	Version  uint32
	MetaSize uint64
	DataSize uint64
	Root     uint32
}

func (h Header) Serialize() []byte {
	var bytes [HeaderSize]byte
	bytes[0] = 'V'
	bytes[1] = 'N'
	bytes[2] = 'G'
	binary.LittleEndian.PutUint32(bytes[4:], h.Version)
	binary.LittleEndian.PutUint64(bytes[8:], h.MetaSize)
	binary.LittleEndian.PutUint64(bytes[16:], h.DataSize)
	binary.LittleEndian.PutUint32(bytes[24:], h.Root)
	return bytes[:]
}

func (h *Header) Deserialize(bytes []byte) error {
	if len(bytes) != HeaderSize || bytes[0] != 'V' || bytes[1] != 'N' || bytes[2] != 'G' || bytes[3] != 0 {
		return errors.New("invalid CSUP header")
	}
	h.Version = binary.LittleEndian.Uint32(bytes[4:])
	h.MetaSize = binary.LittleEndian.Uint64(bytes[8:])
	h.DataSize = binary.LittleEndian.Uint64(bytes[16:])
	h.Root = binary.LittleEndian.Uint32(bytes[24:])
	if h.Version != Version {
		return fmt.Errorf("unsupport CSUP version %d: expected version %d", h.Version, Version)
	}
	if h.MetaSize > MaxMetaSize {
		return fmt.Errorf("CSUP metadata section too big: %d bytes", h.MetaSize)
	}
	if h.MetaSize > MaxDataSize {
		return fmt.Errorf("CSUP data section too big: %d bytes", h.DataSize)
	}
	return nil
}

func (h *Header) ObjectSize() uint64 {
	return HeaderSize + h.MetaSize + h.DataSize
}

func ReadHeader(r io.ReaderAt) (Header, error) {
	var bytes [HeaderSize]byte
	cc, err := r.ReadAt(bytes[:], 0)
	if err != nil {
		return Header{}, err
	}
	if cc < HeaderSize {
		return Header{}, fmt.Errorf("short CSUP file: %d bytes read", cc)
	}
	var h Header
	if err := h.Deserialize(bytes[:]); err != nil {
		return Header{}, err
	}
	return h, nil
}
