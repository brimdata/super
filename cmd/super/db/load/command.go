package load

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"sync/atomic"
	"time"

	"github.com/brimdata/super"
	"github.com/brimdata/super/cli/commitflags"
	"github.com/brimdata/super/cli/dbflags"
	"github.com/brimdata/super/cli/inputflags"
	"github.com/brimdata/super/cli/poolflags"
	"github.com/brimdata/super/cli/runtimeflags"
	"github.com/brimdata/super/cmd/super/db"
	"github.com/brimdata/super/pkg/charm"
	"github.com/brimdata/super/pkg/display"
	"github.com/brimdata/super/pkg/storage"
	"github.com/brimdata/super/pkg/units"
	"github.com/brimdata/super/sio"
	"github.com/paulbellamy/ratecounter"
	"golang.org/x/term"
)

var spec = &charm.Spec{
	Name:  "load",
	Usage: "load [options] file|S3-object|- ...",
	Short: "add and commit data to a branch",
	Long: `
The load command adds data to a pool and commits it to a branch.
`,
	New: New,
}

func init() {
	db.Spec.Add(spec)
}

type Command struct {
	*db.Command
	commitFlags  commitflags.Flags
	inputFlags   inputflags.Flags
	poolFlags    poolflags.Flags
	runtimeFlags runtimeflags.Flags

	// status output
	ctx       context.Context
	rate      *ratecounter.RateCounter
	engine    *engineWrap
	totalRead int64
}

func New(parent charm.Command, f *flag.FlagSet) (charm.Command, error) {
	c := &Command{Command: parent.(*db.Command)}
	c.commitFlags.SetFlags(f)
	c.inputFlags.SetFlags(f, true)
	c.poolFlags.SetFlags(f)
	c.runtimeFlags.SetFlags(f)
	return c, nil
}

func (c *Command) Run(args []string) error {
	ctx, cleanup, err := c.Init(&c.inputFlags, &c.runtimeFlags)
	if err != nil {
		return err
	}
	defer cleanup()
	if len(args) == 0 {
		return errors.New("zed load: at least one input file must be specified (- for stdin)")
	}
	db, err := c.DBFlags.Open(ctx)
	if err != nil {
		return err
	}
	paths := args
	c.engine = &engineWrap{Engine: storage.NewLocalEngine()}
	sctx := super.NewContext()
	readers, err := c.inputFlags.Open(ctx, sctx, c.engine, paths, false)
	if err != nil {
		return err
	}
	defer sio.CloseReaders(readers)
	head, err := c.poolFlags.HEAD()
	if err != nil {
		return err
	}
	if head.Pool == "" {
		return dbflags.ErrNoHEAD
	}
	poolID, err := db.PoolID(ctx, head.Pool)
	if err != nil {
		return err
	}
	var d *display.Display
	if !c.DBFlags.Quiet && term.IsTerminal(int(os.Stderr.Fd())) {
		c.ctx = ctx
		c.rate = ratecounter.NewRateCounter(time.Second)
		d = display.New(c, time.Second/2, os.Stderr)
		go d.Run()
	}
	message := c.commitFlags.CommitMessage()
	commitID, err := db.Load(ctx, sctx, poolID, head.Branch, sio.ConcatReader(readers...), message)
	if d != nil {
		d.Close()
	}
	if err != nil {
		return err
	}
	if !c.DBFlags.Quiet {
		fmt.Printf("%s committed\n", commitID)
	}
	return nil
}

func (c *Command) Display(w io.Writer) bool {
	readBytes, completed := c.engine.status()
	fmt.Fprintf(w, "(%d/%d) ", completed, len(c.engine.readers))
	rate := c.incrRate(readBytes)
	if totalBytes := c.engine.bytesTotal; totalBytes == 0 {
		fmt.Fprintf(w, "%s %s/s\n", readBytes.Abbrev(), rate.Abbrev())
	} else {
		fmt.Fprintf(w, "%s/%s %s/s %.2f%%\n", readBytes.Abbrev(), totalBytes.Abbrev(), rate.Abbrev(), float64(readBytes)/float64(totalBytes)*100)
	}
	return c.ctx.Err() == nil
}

func (c *Command) incrRate(readBytes units.Bytes) units.Bytes {
	c.rate.Incr(int64(readBytes) - c.totalRead)
	c.totalRead = int64(readBytes)
	return units.Bytes(c.rate.Rate())
}

type engineWrap struct {
	storage.Engine
	bytesTotal units.Bytes
	readers    []*byteCounter
	completed  int32
}

func (e *engineWrap) Get(ctx context.Context, u *storage.URI) (storage.Reader, error) {
	r, err := e.Engine.Get(ctx, u)
	if err != nil {
		return nil, err
	}
	size, err := storage.Size(r)
	if err != nil && !errors.Is(err, storage.ErrNotSupported) {
		return nil, err
	}
	e.bytesTotal += units.Bytes(size)
	counter := &byteCounter{Reader: r, completed: &e.completed}
	e.readers = append(e.readers, counter)
	return counter, nil
}

func (e *engineWrap) status() (units.Bytes, int) {
	var read int64
	for _, r := range e.readers {
		read += r.bytesRead()
	}
	return units.Bytes(read), int(atomic.LoadInt32(&e.completed))
}

type byteCounter struct {
	storage.Reader
	n         int64
	completed *int32
}

func (r *byteCounter) Read(b []byte) (int, error) {
	n, err := r.Reader.Read(b)
	atomic.AddInt64(&r.n, int64(n))
	if errors.Is(err, io.EOF) {
		atomic.AddInt32(r.completed, 1)
	}
	return n, err
}

func (r *byteCounter) bytesRead() int64 {
	return atomic.LoadInt64(&r.n)
}
