package typedefs

import (
	"bufio"
	"encoding/binary"
	"errors"
	"flag"
	"fmt"
	"io"

	"github.com/brimdata/super"
	"github.com/brimdata/super/cli/outputflags"
	"github.com/brimdata/super/cmd/super/dev/bsup"
	"github.com/brimdata/super/cmd/super/dev/csup"
	"github.com/brimdata/super/pkg/charm"
	"github.com/brimdata/super/pkg/storage"
	"github.com/brimdata/super/sbuf"
	"github.com/brimdata/super/scode"
	"github.com/brimdata/super/sio"
	"github.com/brimdata/super/sio/bsupio"
	"github.com/brimdata/super/sup"
	"github.com/brimdata/super/vector/vio"
	"github.com/pierrec/lz4/v4"
)

var TypeDefs = &charm.Spec{
	Name:  "typedefs",
	Usage: "typedefs file",
	Short: "read BSUP file and output typedefs",
	Long: `
The typedefs command takes one file argument which must be a BSUP file,
parses each low-level BSUP frame in the file, and outputs the typedefs
from BSUP types frames in any format.`,
	New: New,
}

func init() {
	bsup.Spec.Add(TypeDefs)
}

type Command struct {
	*bsup.Command
	outputFlags outputflags.Flags
}

func New(parent charm.Command, f *flag.FlagSet) (charm.Command, error) {
	c := &Command{Command: parent.(*bsup.Command)}
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
	sctx := super.NewContext()
	meta := newMetaReader(sctx, r)
	if err := vio.Copy(writer, sbuf.NewDematerializer(sctx, sbuf.NewPuller(meta))); err != nil {
		return err
	}
	return writer.Close()
}

type metaReader struct {
	reader    *reader
	marshaler *sup.MarshalBSUPContext
	metas     []any
}

var _ sio.Reader = (*metaReader)(nil)

func newMetaReader(sctx *super.Context, r io.Reader) *metaReader {
	return &metaReader{
		reader:    &reader{reader: bufio.NewReader(r)},
		marshaler: sup.NewBSUPMarshalerWithContext(sctx),
	}
}

func (m *metaReader) Read() (*super.Value, error) {
	if len(m.metas) == 0 {
		bytes, err := m.nextFrame()
		if bytes == nil || err != nil {
			return nil, err
		}
		metas, err := csup.DecodeTypeDefs(bytes)
		if err != nil {
			return nil, err
		}
		m.metas = metas
	}
	meta := m.metas[0]
	m.metas = m.metas[1:]
	val, err := m.marshaler.Marshal(meta)
	return &val, err
}

func (m *metaReader) nextFrame() ([]byte, error) {
	for {
		r := m.reader
		version, err := r.ReadByte()
		if err != nil {
			return nil, noEOF(err)
		}
		if version == 0xff {
			// EOS
			continue
		}
		if err := bsupio.CheckVersion(version); err != nil {
			return nil, err
		}

		code, err := r.ReadByte()
		if err != nil {
			return nil, noEOF(err)
		}
		var bytes []byte
		if (code & 0x40) != 0 {
			bytes, err = r.readComp(code)
			if err != nil {
				return nil, noEOF(err)
			}
		} else {
			bytes, err = r.readUncomp(code)
			if err != nil {
				return nil, noEOF(err)
			}
		}
		switch typ := (code >> 4) & 3; typ {
		case 0:
			// Types frame
			return bytes, nil
		case 1:
			// data - skip
		case 2:
			// ctrl - skip
		default:
			return nil, fmt.Errorf("encountered bad frame type: %d", typ)
		}
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

func (r *reader) readUncomp(code byte) ([]byte, error) {
	size, err := r.readLength(code)
	if err != nil {
		return nil, err
	}
	if size > bsupio.MaxSize {
		return nil, errors.New("BSUP frame too big")
	}
	out := make([]byte, size)
	n, err := r.reader.Read(out)
	if err != nil {
		return nil, err
	}
	if n != len(out) {
		return nil, errors.New("bsupio: short read")
	}
	return out, nil
}

func (r *reader) readComp(code byte) ([]byte, error) {
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
	zlen -= 1 + scode.SizeOfUvarint(uint64(size))

	if format != byte(bsupio.CompressionFormatLZ4) {
		return nil, fmt.Errorf("bsupio: unknown compression format 0x%x", format)
	}
	compressed := make([]byte, zlen)
	n, err := r.reader.Read(compressed)
	if err != nil {
		return nil, err
	}
	if n != len(compressed) {
		return nil, errors.New("bsupio: short read compression byffer")
	}
	uncompressed := make([]byte, size)
	n, err = lz4.UncompressBlock(compressed, uncompressed)
	if err != nil {
		return nil, fmt.Errorf("bsupio: %w", err)
	}
	if n != len(uncompressed) {
		return nil, fmt.Errorf("bsupio: got %d uncompressed bytes, expected %d", n, len(uncompressed))
	}
	return uncompressed, nil
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
