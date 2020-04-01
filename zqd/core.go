package zqd

import (
	"context"
	"net/http"
	"sync"
	"sync/atomic"

	"github.com/brimsec/zq/zqd/zeek"
	"go.uber.org/zap"
)

type Config struct {
	Root string
	// ZeekLauncher is the interface for launching zeek processes.
	ZeekLauncher zeek.Launcher
	// SortLimit specifies the limit of logs in posted pcap to sort. Its
	// existence is only as a hook for testing.  Eventually zqd will sort an
	// unlimited amount of logs and this can be taken out.
	SortLimit int
	Logger    *zap.Logger
}

type VersionMessage struct {
	Zqd string `json:"boomd"` //XXX boomd -> zqd
	Zq  string `json:"zq"`
}

// This struct filled in by main from linker setting version strings.
var Version VersionMessage

type Core struct {
	Root         string
	ZeekLauncher zeek.Launcher
	// SortLimit specifies the limit of logs in posted pcap to sort. Its
	// existence is only as a hook for testing.  Eventually zqd will sort an
	// unlimited amount of logs and this can be taken out.
	SortLimit int
	taskCount int64
	logger    *zap.Logger

	// spaceOpsLock protects the spaceOps map and the currentOps and
	// deletePending fields inside the spaceOpsState's.
	spaceOpsLock sync.Mutex
	spaceOps     map[string]*spaceOpsState
}

type spaceOpsState struct {
	currentOps    int
	deletePending int

	wg sync.WaitGroup
	// closed to signal non-delete ops should terminate
	cancelChan chan struct{}
}

func NewCore(conf Config) *Core {
	logger := conf.Logger
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Core{
		Root:         conf.Root,
		ZeekLauncher: conf.ZeekLauncher,
		SortLimit:    conf.SortLimit,
		logger:       logger,
		spaceOps:     make(map[string]*spaceOpsState),
	}
}

func (c *Core) HasZeek() bool {
	return c.ZeekLauncher != nil
}

func (c *Core) requestLogger(r *http.Request) *zap.Logger {
	return c.logger.With(zap.String("request_id", getRequestID(r.Context())))
}

func (c *Core) getTaskID() int64 {
	return atomic.AddInt64(&c.taskCount, 1)
}

// startSpaceOp registers that an operation on a space is in progress.
// If the space is pending deletion, the bool parameter returns false.
// Otherwise, this returns a new context, and a done function that must
// be called when the operation completes.
func (c *Core) startSpaceOp(ctx context.Context, space string) (context.Context, func(), bool) {
	c.spaceOpsLock.Lock()
	defer c.spaceOpsLock.Unlock()

	state, ok := c.spaceOps[space]
	if !ok {
		state = &spaceOpsState{
			cancelChan: make(chan struct{}, 0),
		}
		c.spaceOps[space] = state
	}
	if state.deletePending > 0 {
		return ctx, func() {}, false
	}
	state.currentOps++
	state.wg.Add(1)

	ctx, cancel := context.WithCancel(ctx)
	go func() {
		select {
		case <-ctx.Done():
		case <-state.cancelChan:
			cancel()
		}
	}()

	ingestDone := func() {
		c.spaceOpsLock.Lock()
		state.currentOps--
		if state.currentOps == 0 && state.deletePending == 0 {
			delete(c.spaceOps, space)
		}
		c.spaceOpsLock.Unlock()

		state.wg.Done()
		cancel()
	}

	return ctx, ingestDone, true
}

// haltSpaceOpsForDelete signals any outstanding operations that registered with
// startSpaceOp to halt and marks the space as pending delete. It returns a done
// function that must be called when the delete operation completes.
func (c *Core) haltSpaceOpsForDelete(space string) func() {
	c.spaceOpsLock.Lock()

	state, ok := c.spaceOps[space]
	if !ok {
		state = &spaceOpsState{
			cancelChan: make(chan struct{}, 0),
		}
		c.spaceOps[space] = state
	}
	if state.deletePending == 0 {
		close(state.cancelChan)
	}
	state.deletePending++

	c.spaceOpsLock.Unlock()

	state.wg.Wait()

	return func() {
		c.spaceOpsLock.Lock()
		defer c.spaceOpsLock.Unlock()
		state.deletePending--
		if state.deletePending == 0 {
			delete(c.spaceOps, space)
		}
	}
}
