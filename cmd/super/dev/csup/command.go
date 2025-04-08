package csup

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"

	"github.com/brimdata/super"
	"github.com/brimdata/super/cli/outputflags"
	"github.com/brimdata/super/cmd/super/dev"

	"github.com/brimdata/super/csup"
	"github.com/brimdata/super/pkg/charm"
	"github.com/brimdata/super/pkg/storage"
	"github.com/brimdata/super/sup"
	"github.com/brimdata/super/zio"
	"github.com/brimdata/super/zio/bsupio"
)

var spec = &charm.Spec{
	Name:  "csup",
	Usage: "csup uri",
	Short: "dump CSUP metadata",
	Long: `
csup decodes an input uri and emits the metadata sections in the format desired.`,
	New: New,
}

func init() {
	dev.Spec.Add(spec)
}

type Command struct {
	*dev.Command
	outputFlags outputflags.Flags
}

func New(parent charm.Command, f *flag.FlagSet) (charm.Command, error) {
	c := &Command{Command: parent.(*dev.Command)}
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
		return errors.New("a single file is required")
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
	meta := newReader(r)
	err = zio.Copy(writer, meta)
	if err2 := writer.Close(); err == nil {
		err = err2
	}
	return err
}

type reader struct {
	sctx      *super.Context
	reader    *bufio.Reader
	meta      *bsupio.Reader
	marshaler *sup.MarshalBSUPContext
	dataSize  int
}

var _ zio.Reader = (*reader)(nil)

func newReader(r io.Reader) *reader {
	sctx := super.NewContext()
	return &reader{
		sctx:      sctx,
		reader:    bufio.NewReader(r),
		marshaler: sup.NewBSUPMarshalerWithContext(sctx),
	}
}

func (r *reader) Read() (*super.Value, error) {
	for {
		if r.meta == nil {
			hdr, err := r.readHeader()
			if err != nil {
				if err == io.EOF {
					err = nil
				}
				return nil, err
			}
			r.meta = bsupio.NewReader(r.sctx, io.LimitReader(r.reader, int64(hdr.MetaSize)))
			r.dataSize = int(hdr.DataSize)
			val, err := r.marshaler.Marshal(hdr)
			return val.Ptr(), err
		}
		val, err := r.meta.Read()
		if val != nil || err != nil {
			return val, err
		}
		if err := r.meta.Close(); err != nil {
			return nil, err
		}
		r.meta = nil
		r.skip(r.dataSize)
	}
}

func (r *reader) readHeader() (csup.Header, error) {
	var bytes [csup.HeaderSize]byte
	cc, err := r.reader.Read(bytes[:])
	if err != nil {
		return csup.Header{}, err
	}
	if cc != csup.HeaderSize {
		return csup.Header{}, fmt.Errorf("truncated CSUP file: %d bytes of %d read", cc, csup.HeaderSize)
	}
	var h csup.Header
	if err := h.Deserialize(bytes[:]); err != nil {
		return csup.Header{}, err
	}
	return h, nil
}

func (r *reader) skip(n int) error {
	got, err := r.reader.Discard(n)
	if n != got {
		return fmt.Errorf("truncated CSUP data: data section %d but read only %d", n, got)
	}
	return err
}
