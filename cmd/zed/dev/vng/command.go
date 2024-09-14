package vng

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"

	"github.com/brimdata/zed"
	"github.com/brimdata/zed/cli/outputflags"
	"github.com/brimdata/zed/cmd/zed/dev"
	"github.com/brimdata/zed/cmd/zed/root"
	"github.com/brimdata/zed/pkg/charm"
	"github.com/brimdata/zed/pkg/storage"
	"github.com/brimdata/zed/vng"
	"github.com/brimdata/zed/zio"
	"github.com/brimdata/zed/zio/zngio"
	"github.com/brimdata/zed/zson"
)

var Cmd = &charm.Spec{
	Name:  "vng",
	Usage: "vng uri",
	Short: "dump VNG metadata",
	Long: `
vng decodes an input uri and emits the metadata sections in the format desired.`,
	New: New,
}

func init() {
	dev.Cmd.Add(Cmd)
}

type Command struct {
	*root.Command
	outputFlags outputflags.Flags
}

func New(parent charm.Command, f *flag.FlagSet) (charm.Command, error) {
	c := &Command{Command: parent.(*root.Command)}
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
	zctx      *zed.Context
	reader    *bufio.Reader
	meta      *zngio.Reader
	marshaler *zson.MarshalZNGContext
	dataSize  int
}

var _ zio.Reader = (*reader)(nil)

func newReader(r io.Reader) *reader {
	zctx := zed.NewContext()
	return &reader{
		zctx:      zctx,
		reader:    bufio.NewReader(r),
		marshaler: zson.NewZNGMarshalerWithContext(zctx),
	}
}

func (r *reader) Read() (*zed.Value, error) {
	for {
		if r.meta == nil {
			hdr, err := r.readHeader()
			if err != nil {
				if err == io.EOF {
					err = nil
				}
				return nil, err
			}
			r.meta = zngio.NewReader(r.zctx, io.LimitReader(r.reader, int64(hdr.MetaSize)))
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

func (r *reader) readHeader() (vng.Header, error) {
	var bytes [vng.HeaderSize]byte
	cc, err := r.reader.Read(bytes[:])
	if err != nil {
		return vng.Header{}, err
	}
	if cc != vng.HeaderSize {
		return vng.Header{}, fmt.Errorf("truncated VNG file: %d bytes of %d read", cc, vng.HeaderSize)
	}
	var h vng.Header
	if err := h.Deserialize(bytes[:]); err != nil {
		return vng.Header{}, err
	}
	return h, nil
}

func (r *reader) skip(n int) error {
	got, err := r.reader.Discard(n)
	if n != got {
		return fmt.Errorf("truncated VNG data: data section %d but read only %d", n, got)
	}
	return err
}
