package typedefs

import (
	"bufio"
	"encoding/binary"
	"errors"
	"flag"
	"fmt"
	"io"

	"github.com/pierrec/lz4/v4"
	"github.com/superdb/super"
	"github.com/superdb/super/bsup/rows"
	"github.com/superdb/super/cli/outputflags"
	"github.com/superdb/super/cmd/super/dev/bsup"
	"github.com/superdb/super/cmd/super/dev/bsuprows"
	"github.com/superdb/super/pkg/charm"
	"github.com/superdb/super/pkg/storage"
	"github.com/superdb/super/sbuf"
	"github.com/superdb/super/scode"
	"github.com/superdb/super/sio"
	"github.com/superdb/super/vector/vio"
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
	bsuprows.Spec.Add(TypeDefs)
}

type Command struct {
	*bsuprows.Command
	outputFlags outputflags.Flags
}

func New(parent charm.Command, f *flag.FlagSet) (charm.Command, error) {
	c := &Command{Command: parent.(*bsuprows.Command)}
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
	marshaler *super.Marshaler
	metas     []any
	off       int
}

var _ sio.Reader = (*metaReader)(nil)

func newMetaReader(sctx *super.Context, r io.Reader) *metaReader {
	return &metaReader{
		reader:    &reader{reader: bufio.NewReader(r)},
		marshaler: super.NewMarshaler(sctx),
	}
}

func (m *metaReader) Read() (*super.Value, error) {
	if len(m.metas) == 0 {
		bytes, err := m.nextFrame()
		if bytes == nil || err != nil {
			return nil, err
		}
		metas, err := bsup.DecodeTypeDefs(bytes, m.off)
		if err != nil {
			return nil, err
		}
		m.metas = metas
		m.off += len(metas)
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
			m.off = 0
			continue
		}
		if err := rows.CheckVersion(version); err != nil { //XXX
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
	if size > rows.MaxSize { //XXX
		return nil, errors.New("BSUP rows frame too big")
	}
	out := make([]byte, size)
	n, err := io.ReadFull(r.reader, out)
	if err != nil {
		return nil, err
	}
	if n != len(out) {
		return nil, errors.New("BSUP rows: short read")
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

	if format != byte(rows.CompressionFormatLZ4) { //XXX
		return nil, fmt.Errorf("BSUP rows: unknown compression format 0x%x", format)
	}
	compressed := make([]byte, zlen)
	n, err := io.ReadFull(r.reader, compressed)
	if err != nil {
		return nil, err
	}
	if n != len(compressed) {
		return nil, fmt.Errorf("BSUP rows: short read compression buffer (%d of %d)", n, zlen)
	}
	uncompressed := make([]byte, size)
	n, err = lz4.UncompressBlock(compressed, uncompressed)
	if err != nil {
		return nil, fmt.Errorf("BSUP rows: %w", err)
	}
	if n != len(uncompressed) {
		return nil, fmt.Errorf("BSUP rows: got %d uncompressed bytes, expected %d", n, len(uncompressed))
	}
	return uncompressed, nil
}

func (r *reader) skip(n int) error {
	if n > 25*1024*1024 {
		return fmt.Errorf("BSUP rows: buffer length too big: %d", n)
	}
	got, err := r.reader.Discard(n)
	if n != got {
		return fmt.Errorf("BSUP rows: short read: wanted to discard %d but got only %d", n, got)
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
