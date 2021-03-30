package ingest

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/brimdata/zed/api"
	"github.com/brimdata/zed/compiler"
	"github.com/brimdata/zed/driver"
	"github.com/brimdata/zed/pkg/fs"
	"github.com/brimdata/zed/pkg/iosrc"
	"github.com/brimdata/zed/pkg/nano"
	"github.com/brimdata/zed/ppl/zqd/pcapanalyzer"
	"github.com/brimdata/zed/ppl/zqd/pcapstorage"
	"github.com/brimdata/zed/ppl/zqd/storage"
	"github.com/brimdata/zed/ppl/zqd/storage/archivestore"
	"github.com/brimdata/zed/zbuf"
	"github.com/brimdata/zed/zio"
	"github.com/brimdata/zed/zio/detector"
	"github.com/brimdata/zed/zio/zngio"
	"github.com/brimdata/zed/zng/resolver"
)

type PcapOp interface {
	Status() api.PcapPostStatus
	// Err returns the an error if an error occurred while the ingest process was
	// running. If the process is still running Err will wait for the process to
	// complete before returning.
	Err() error
	Warn() <-chan string
	Done() <-chan struct{}
	Snap() <-chan struct{}
}

var suricataShaper = compiler.MustParseProc(
	`type alert = {event_type:bstring,src_ip:ip,src_port:port=(uint16),dest_ip:ip,dest_port:port=(uint16),vlan:[uint16],proto:bstring,app_proto:bstring,alert:{severity:uint16,signature:bstring,category:bstring,action:bstring,signature_id:uint64,gid:uint64,rev:uint64,metadata:{signature_severity:[bstring],former_category:[bstring],attack_target:[bstring],deployment:[bstring],affected_product:[bstring],created_at:[bstring],performance_impact:[bstring],updated_at:[bstring],malware_family:[bstring],tag:[bstring]}},flow_id:uint64,pcap_cnt:uint64,timestamp:time,tx_id:uint64,icmp_code:uint64,icmp_type:uint64,tunnel:{src_ip:ip,src_port:port=(uint16),dest_ip:ip,dest_port:port=(uint16),proto:bstring,depth:uint64},community_id:bstring}

put . = shape(alert) | rename ts=timestamp
`)

type ClearableStore interface {
	storage.Storage
	Clear(ctx context.Context) error
}

// NewPcapOp kicks of the process for ingesting a pcap file into a space.
func NewPcapOp(ctx context.Context, store storage.Storage, pcapstore *pcapstorage.Store, pcap string, suricata, zeek pcapanalyzer.Launcher) (PcapOp, error) {
	pcapuri, err := iosrc.ParseURI(pcap)
	if err != nil {
		return nil, err
	}
	if suricata == nil && zeek == nil {
		return nil, fmt.Errorf("must provide at least one launcher")
	}
	logstore, ok := store.(ClearableStore)
	if ok {
		return newFilePcapOp(ctx, pcapstore, logstore, pcapuri, suricata, zeek)
	}
	return newArchivePcapOp(ctx, store.(*archivestore.Storage), pcapstore, pcapuri, suricata, zeek)
}

type legacyPcapOp struct {
	pcapstore  *pcapstorage.Store
	store      ClearableStore
	snapshots  int32
	pcapuri    iosrc.URI
	logdir     string
	done, snap chan struct{}
	err        error
	warn       chan string

	slauncher, zlauncher pcapanalyzer.Launcher

	// stats
	startTime    nano.Ts
	pcapSize     int64
	pcapReadSize int64
}

func newFilePcapOp(ctx context.Context, pcapstore *pcapstorage.Store, store ClearableStore, pcapuri iosrc.URI, slauncher, zlauncher pcapanalyzer.Launcher) (*legacyPcapOp, error) {
	if slauncher == nil && zlauncher == nil {
		return nil, fmt.Errorf("must provide at least one launcher")
	}
	info, err := iosrc.Stat(ctx, pcapuri)
	if err != nil {
		return nil, err
	}
	warn := make(chan string)
	go func() {
		err = pcapstore.Update(ctx, pcapuri, warn)
		close(warn)
	}()
	var warnings []string
	for w := range warn {
		warnings = append(warnings, w)
	}
	if err != nil {
		return nil, err
	}
	logdir, err := ioutil.TempDir("", "zqd-pcap-ingest-")
	if err != nil {
		return nil, err
	}
	p := &legacyPcapOp{
		startTime: nano.Now(),
		pcapSize:  info.Size(),
		pcapstore: pcapstore,
		store:     store,
		pcapuri:   pcapuri,
		logdir:    logdir,
		done:      make(chan struct{}),
		snap:      make(chan struct{}),
		warn:      make(chan string),
		slauncher: slauncher,
		zlauncher: zlauncher,
	}
	go func() {
		for _, w := range warnings {
			p.warn <- w
		}
		p.err = p.run(ctx)
		close(p.done)
		close(p.snap)
	}()
	return p, nil
}

func (p *legacyPcapOp) run(ctx context.Context) error {
	var sErr, zErr error
	var wg sync.WaitGroup
	slurpDone := make(chan struct{})
	if p.slauncher != nil {
		wg.Add(1)
		go func() {
			sErr = p.runSuricata(ctx)
			wg.Done()
		}()
	}
	if p.zlauncher != nil {
		wg.Add(1)
		go func() {
			zErr = p.runZeek(ctx)
			wg.Done()
		}()
	}
	go func() {
		wg.Wait()
		close(slurpDone)
	}()

	abort := func() {
		os.RemoveAll(p.logdir)
		// Don't want to use passed context here because a cancelled context
		// would cause storage not to be cleared.
		p.pcapstore.Delete(context.Background())
		p.store.Clear(context.Background())
	}

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	start := time.Now()
	next := time.Second
outer:
	for {
		select {
		case <-slurpDone:
			break outer
		case t := <-ticker.C:
			if t.After(start.Add(next)) {
				if err := p.createSnapshot(ctx); err != nil {
					abort()
					return err
				}
				select {
				case p.snap <- struct{}{}:
				default:
				}
				next = 2 * next
			}
		}
	}
	if sErr != nil {
		abort()
		return sErr
	}
	if zErr != nil {
		abort()
		return zErr
	}

	if err := p.createSnapshot(ctx); err != nil {
		abort()
		return err
	}
	if err := os.RemoveAll(p.logdir); err != nil {
		abort()
		return err
	}
	return nil
}

func (p *legacyPcapOp) runZeek(ctx context.Context) error {
	pcapfile, err := iosrc.NewReader(ctx, p.pcapuri)
	if err != nil {
		return err
	}
	defer pcapfile.Close()
	r := io.TeeReader(pcapfile, p)
	zproc, err := p.zlauncher(ctx, bufio.NewReader(r), p.logdir)
	if err != nil {
		return err
	}
	return zproc.Wait()
}

func (p *legacyPcapOp) runSuricata(ctx context.Context) error {
	pcapfile, err := iosrc.NewReader(ctx, p.pcapuri)
	if err != nil {
		return err
	}
	defer pcapfile.Close()
	sproc, err := p.slauncher(ctx, bufio.NewReader(pcapfile), p.logdir)
	if err != nil {
		return err
	}
	if err = sproc.Wait(); err != nil {
		return err
	}
	return p.convertSuricataLog(ctx)
}

func (p *legacyPcapOp) Status() api.PcapPostStatus {
	return api.PcapPostStatus{
		Type:          "PcapPostStatus",
		StartTime:     p.startTime,
		UpdateTime:    nano.Now(),
		PcapSize:      p.pcapSize,
		PcapReadSize:  atomic.LoadInt64(&p.pcapReadSize),
		SnapshotCount: int(atomic.LoadInt32(&p.snapshots)),
	}
}

// Err returns the an error if an error occurred while the ingest process was
// running. If the process is still running Err will wait for the process to
// complete before returning.
func (p *legacyPcapOp) Err() error {
	<-p.done
	return p.err
}

// Done returns a chan that emits when the ingest process is complete.
func (p *legacyPcapOp) Done() <-chan struct{} {
	return p.done
}

// Warn returns a chan that emits warnings.
func (p *legacyPcapOp) Warn() <-chan string {
	return p.warn
}

func (p *legacyPcapOp) SnapshotCount() int {
	return int(atomic.LoadInt32(&p.snapshots))
}

// Snap returns a chan that emits every time a snapshot is made. It
// should no longer be read from after Done() has emitted.
func (p *legacyPcapOp) Snap() <-chan struct{} {
	return p.snap
}

func (p *legacyPcapOp) zeekFiles() []string {
	files, err := filepath.Glob(filepath.Join(p.logdir, "*.log"))
	// Per filepath.Glob documentation the only possible error would be due to
	// an invalid glob pattern. Ok to panic.
	if err != nil {
		panic(err)
	}
	return files
}

func (p *legacyPcapOp) suricataFiles() []string {
	path := filepath.Join(p.logdir, "eve.zng")
	if _, err := os.Stat(path); err != nil {
		return nil
	}
	return []string{path}
}

func (p *legacyPcapOp) createSnapshot(ctx context.Context) error {
	files := append(p.zeekFiles(), p.suricataFiles()...)
	if len(files) == 0 {
		return nil
	}
	// convert logs into sorted zng
	zctx := resolver.NewContext()
	readers, err := detector.OpenFiles(ctx, zctx, files...)
	if err != nil {
		return err
	}
	defer zbuf.CloseReaders(readers)
	reader, err := zbuf.MergeReadersByTsAsReader(ctx, readers, p.store.NativeOrder())
	if err != nil {
		return err
	}
	if err := p.store.Write(ctx, zctx, reader); err != nil {
		return err
	}
	atomic.AddInt32(&p.snapshots, 1)
	return nil
}

func (p *legacyPcapOp) convertSuricataLog(ctx context.Context) error {
	zctx := resolver.NewContext()
	path := filepath.Join(p.logdir, "eve.json")
	zr, err := detector.OpenFile(zctx, path, zio.ReaderOpts{})
	if err != nil {
		return err
	}
	defer zr.Close()
	return fs.ReplaceFile(filepath.Join(p.logdir, "eve.zng"), os.FileMode(0666), func(w io.Writer) error {
		zw := zngio.NewWriter(zio.NopCloser(w), zngio.WriterOpts{})
		return driver.Copy(ctx, zw, suricataShaper, zctx, zr, driver.Config{})
	})
}

func (p *legacyPcapOp) Write(b []byte) (int, error) {
	n := len(b)
	atomic.AddInt64(&p.pcapReadSize, int64(n))
	return n, nil
}
