package frames

import (
	"bufio"
	"encoding/binary"
	"errors"
	"flag"
	"fmt"
	"io"

	"github.com/brimdata/zed"
	"github.com/brimdata/zed/cli/outputflags"
	"github.com/brimdata/zed/cmd/zed/dev/dig"
	"github.com/brimdata/zed/pkg/charm"
	"github.com/brimdata/zed/pkg/storage"
	"github.com/brimdata/zed/zcode"
	"github.com/brimdata/zed/zio"
	"github.com/brimdata/zed/zson"
)

var Frames = &charm.Spec{
	Name:  "frames",
	Usage: "frames file",
	Short: "read ZNG file and output metadata",
	Long: `
The frames command takes one file arugmnet which must be a ZNG file,
parses each low-level ZNG frame in the file, and outputs meta describing each frame
in any Zed format.`,
	New: New,
}

func init() {
	dig.Cmd.Add(Frames)
}

type Command struct {
	*dig.Command
	outputFlags outputflags.Flags
}

func New(parent charm.Command, f *flag.FlagSet) (charm.Command, error) {
	c := &Command{Command: parent.(*dig.Command)}
	c.outputFlags.SetFlags(f)
	return c, nil
}

func (c *Command) Run(args []string) error {
	ctx, cleanup, err := c.Init(&c.outputFlags)
	if err != nil {
		return err
	}
	defer cleanup()
	if len(args) != 1 {
		return errors.New("a single file required")
	}
	uri, err := storage.ParseURI(args[0])
	if err != nil {
		return err
	}
	engine := storage.NewLocalEngine()
	r, err := engine.Get(ctx, uri)
	if err != nil {
		return err
	}
	defer r.Close()
	writer, err := c.outputFlags.Open(ctx, engine)
	if err != nil {
		return err
	}
	meta := newMetaReader(r)
	if err := zio.Copy(writer, meta); err != nil {
		return err
	}
	return writer.Close()
}

type metaReader struct {
	reader    *reader
	marshaler *zson.MarshalZNGContext
}

var _ zio.Reader = (*metaReader)(nil)

func newMetaReader(r io.Reader) *metaReader {
	return &metaReader{
		reader:    &reader{reader: bufio.NewReader(r)},
		marshaler: zson.NewZNGMarshaler(),
	}
}

type EOS struct {
	Type   string `zed:"type"`
	Offset int64  `zed:"offset"`
}

type Frame struct {
	Type   string      `zed:"type"`
	Offset int64       `zed:"offset"`
	Block  interface{} `zed:"block"`
}

type UncompressedBlock struct {
	Type   string `zed:"type"`
	Length int64  `zed:"length"`
}

type CompressedBlock struct {
	Type   string `zed:"type"`
	Length int64  `zed:"length"`
	Format int8   `zed:"format"`
	Size   int64  `zed:"size"`
}

func (m *metaReader) Read() (*zed.Value, error) {
	f, err := m.nextFrame()
	if f == nil || err != nil {
		return nil, err
	}
	val, err := m.marshaler.Marshal(f)
	return &val, err
}

func (m *metaReader) nextFrame() (interface{}, error) {
	r := m.reader
	pos := r.pos
	code, err := r.ReadByte()
	if err != nil {
		return nil, noEOF(err)
	}
	if code == 0xff {
		return &Frame{Type: "EOS", Offset: pos}, nil

	}
	if (code & 0x80) != 0 {
		return nil, errors.New("encountered wrong version bit in ZNG framing")
	}
	var block interface{}
	if (code & 0x40) != 0 {
		block, err = r.readComp(code)
		if err != nil {
			return nil, noEOF(err)
		}
	} else {
		block, err = r.readUncomp(code)
		if err != nil {
			return nil, noEOF(err)
		}
	}
	switch typ := (code >> 4) & 3; typ {
	case 0:
		return &Frame{Type: "types", Offset: pos, Block: block}, nil
	case 1:
		return &Frame{Type: "values", Offset: pos, Block: block}, nil
	case 2:
		return &Frame{Type: "control", Offset: pos, Block: block}, nil
	default:
		return nil, fmt.Errorf("encountered bad frame type: %d", typ)
	}
}

type reader struct {
	reader *bufio.Reader
	pos    int64
}

func (r *reader) ReadByte() (byte, error) {
	code, err := r.reader.ReadByte()
	if err != nil {
		return 0, err
	}
	r.pos++
	return code, nil
}

func (r *reader) readUncomp(code byte) (interface{}, error) {
	size, err := r.readLength(code)
	if err != nil {
		return 0, err
	}
	return &UncompressedBlock{
		Type:   "uncompressed",
		Length: int64(size),
	}, r.skip(size)
}

func (r *reader) readComp(code byte) (interface{}, error) {
	zlen, err := r.readLength(code)
	if err != nil {
		return nil, err
	}
	format, err := r.ReadByte()
	if err != nil {
		return nil, err
	}
	size, err := r.readUvarint()
	if err != nil {
		return nil, err
	}
	// The size of the compressed buffer needs to be adjusted by the
	// byte for the format and the variable-length bytes to encode
	// the original size.
	zlen -= 1 + zcode.SizeOfUvarint(uint64(size))
	err = r.skip(zlen)
	if err != nil && err != io.EOF {
	}
	return &CompressedBlock{
		Type:   "compressed",
		Length: int64(zlen),
		Format: int8(format),
		Size:   int64(size),
	}, nil
}

func (r *reader) skip(n int) error {
	if n > 25*1024*1024 {
		return fmt.Errorf("buffer length too big: %d", n)
	}
	got, err := r.reader.Discard(n)
	if n != got {
		return fmt.Errorf("short read: wanted to discard %d but got only %d", n, got)
	}
	r.pos += int64(n)
	return err
}

func (r *reader) readLength(code byte) (int, error) {
	v, err := r.readUvarint()
	if err != nil {
		return 0, err
	}
	return (v << 4) | (int(code) & 0xf), nil
}

func (r *reader) readUvarint() (int, error) {
	u64, err := binary.ReadUvarint(r)
	return int(u64), err
}

func noEOF(err error) error {
	if err == io.EOF {
		err = nil
	}
	return err
}
