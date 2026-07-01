package csup

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
)

const (
	Version        = 23
	HeaderSize     = 9
	DataHeaderSize = 36
	FooterSize     = 4
	TrailerSize    = 16
	MaxMetaSize    = 100 * 1024 * 1024
	MaxTypeSize    = 100 * 1024 * 1024
	MaxDataSize    = 2 * 1024 * 1024 * 1024
)

type SectionType byte

const (
	SectionObject SectionType = 'O'
	SectionFooter SectionType = 'F'
)

type Section struct {
	Type   SectionType
	Object DataHeader
	Footer Footer
}

func ReadSection(r io.ReaderAt) (Section, error) {
	h, err := ReadHeader(r)
	if err != nil {
		return Section{}, err
	}
	s := Section{Type: h.SectionType}
	switch h.SectionType {
	case SectionObject:
		s.Object, err = ReadDataHeader(io.NewSectionReader(r, int64(h.Size()), math.MaxInt64))
	case SectionFooter:
		s.Footer, err = ReadFooter(io.NewSectionReader(r, int64(h.Size()), math.MaxInt64))
	default:
		panic(fmt.Sprintf("invalid CSUP section type: %c", h.SectionType))
	}
	if err == io.EOF {
		err = io.ErrUnexpectedEOF
	}
	return s, err
}

func (s *Section) Size() uint64 {
	if s.Type == SectionObject {
		return HeaderSize + s.Object.Size()
	}
	return HeaderSize + s.Footer.Size()
}

type Header struct {
	Version     uint32
	SectionType SectionType
}

func (o Header) Serialize() []byte {
	var bytes [HeaderSize]byte
	bytes[0] = 'C'
	bytes[1] = 'S'
	bytes[2] = 'U'
	bytes[3] = 'P'
	binary.LittleEndian.PutUint32(bytes[4:], o.Version)
	bytes[8] = byte(o.SectionType)
	return bytes[:]
}

func (o *Header) Deserialize(bytes []byte) error {
	if len(bytes) != HeaderSize || bytes[0] != 'C' || bytes[1] != 'S' || bytes[2] != 'U' || bytes[3] != 'P' {
		return errors.New("invalid CSUP header")
	}
	o.Version = binary.LittleEndian.Uint32(bytes[4:])
	o.SectionType = SectionType(bytes[8])
	if o.Version != Version {
		return fmt.Errorf("CSUP version mismatch: expected %d, found %d", Version, o.Version)
	}
	if o.SectionType != SectionObject && o.SectionType != SectionFooter {
		return fmt.Errorf("invalid CSUP section type %c", o.SectionType)
	}
	return nil
}

func (o *Header) Size() uint64 { return HeaderSize }

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

type DataHeader struct {
	MetaSize uint64
	TypeSize uint64
	DataSize uint64
	Root     uint32
}

func ReadDataHeader(r io.ReaderAt) (DataHeader, error) {
	var bytes [DataHeaderSize]byte
	n, err := r.ReadAt(bytes[:], 0)
	if err != nil {
		return DataHeader{}, err
	}
	if n < DataHeaderSize {
		return DataHeader{}, fmt.Errorf("short CSUP object header: %d bytes read", n)
	}
	var h DataHeader
	if err := h.Deserialize(bytes[:]); err != nil {
		return DataHeader{}, err
	}
	return h, nil
}

func (o *DataHeader) Size() uint64 {
	return DataHeaderSize + o.MetaSize + o.TypeSize + o.DataSize
}

func (o DataHeader) Serialize() []byte {
	var bytes [DataHeaderSize]byte
	binary.LittleEndian.PutUint64(bytes[:], o.MetaSize)
	binary.LittleEndian.PutUint64(bytes[8:], o.TypeSize)
	binary.LittleEndian.PutUint64(bytes[16:], o.DataSize)
	binary.LittleEndian.PutUint32(bytes[24:], o.Root)
	return bytes[:]
}

func (o *DataHeader) Deserialize(bytes []byte) error {
	o.MetaSize = binary.LittleEndian.Uint64(bytes)
	o.TypeSize = binary.LittleEndian.Uint64(bytes[8:])
	o.DataSize = binary.LittleEndian.Uint64(bytes[16:])
	o.Root = binary.LittleEndian.Uint32(bytes[24:])
	if o.MetaSize > MaxMetaSize {
		return fmt.Errorf("CSUP metadata section too big: %d bytes", o.MetaSize)
	}
	if o.MetaSize > MaxTypeSize {
		return fmt.Errorf("CSUP type section too big: %d bytes", o.TypeSize)
	}
	if o.DataSize > MaxDataSize {
		return fmt.Errorf("CSUP data section too big: %d bytes", o.DataSize)
	}
	return nil
}

type Footer struct {
	MetaSize uint32
}

func ReadFooter(r io.ReaderAt) (Footer, error) {
	var bytes [FooterSize]byte
	cc, err := r.ReadAt(bytes[:], 0)
	if err != nil {
		return Footer{}, err
	}
	if cc < FooterSize {
		return Footer{}, fmt.Errorf("short CSUP footer: %d bytes read", cc)
	}
	var f Footer
	f.Deserialize(bytes[:])
	return f, nil
}

func (f Footer) Serialize() []byte {
	var bytes [FooterSize]byte
	binary.LittleEndian.PutUint32(bytes[:], f.MetaSize)
	return bytes[:]
}

func (f *Footer) Deserialize(bytes []byte) {
	f.MetaSize = binary.LittleEndian.Uint32(bytes)
}

func (f *Footer) Size() uint64 {
	return FooterSize + uint64(f.MetaSize) + TrailerSize
}

type Trailer struct {
	Size     uint64
	MetaSize uint32
}

func ReadTrailer(r io.ReaderAt) (Trailer, error) {
	var bytes [TrailerSize]byte
	cc, err := r.ReadAt(bytes[:], 0)
	if err != nil {
		return Trailer{}, err
	}
	if cc < TrailerSize {
		return Trailer{}, fmt.Errorf("short CSUP trailer: %d bytes read", cc)
	}
	var t Trailer
	t.Deserialize(bytes[:])
	return t, nil
}

func (t Trailer) Serialize() []byte {
	var bytes [TrailerSize]byte
	binary.LittleEndian.PutUint64(bytes[:], t.Size)
	binary.LittleEndian.PutUint32(bytes[8:], t.MetaSize)
	bytes[12] = 'C'
	bytes[13] = 'S'
	bytes[14] = 'U'
	bytes[15] = 'P'
	return bytes[:]
}

func (t *Trailer) Deserialize(bytes []byte) error {
	if len(bytes) != TrailerSize || string(bytes[12:16]) != "CSUP" {
		return errors.New("invalid CSUP trailer")
	}
	t.Size = binary.LittleEndian.Uint64(bytes)
	t.MetaSize = binary.LittleEndian.Uint32(bytes[8:])
	return nil
}
