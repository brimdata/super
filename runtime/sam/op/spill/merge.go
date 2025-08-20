package spill

import (
	"container/heap"
	"context"
	"os"
	"path/filepath"
	"strconv"

	"github.com/brimdata/super"
	"github.com/brimdata/super/runtime/sam/expr"
	"github.com/brimdata/super/sio"
)

// MergeSort manages "runs" (files of sorted BSUP values) that are spilled to
// disk a chunk at a time, then read back and merged in sorted order, effectively
// implementing an external merge sort.
type MergeSort struct {
	comparator *expr.Comparator
	nspill     int
	runs       []*peeker
	tempDir    string
	spillSize  int64
	sctx       *super.Context
}

const TempPrefix = "zed-spill-"

func TempDir() (string, error) {
	return os.MkdirTemp("", TempPrefix)
}

func TempFile() (*os.File, error) {
	return os.CreateTemp("", TempPrefix)
}

// NewMergeSort returns a MergeSort to implement external merge sorts of a large
// BSUP stream.  It creates a temporary directory to hold the collection
// of spilled chunks.  Call Cleanup to remove it.
func NewMergeSort(comparator *expr.Comparator) (*MergeSort, error) {
	tempDir, err := TempDir()
	if err != nil {
		return nil, err
	}
	return &MergeSort{
		comparator: comparator,
		tempDir:    tempDir,
		sctx:       super.NewContext(),
	}, nil
}

func (r *MergeSort) Cleanup() {
	for _, run := range r.runs {
		run.CloseAndRemove()
	}
	os.RemoveAll(r.tempDir)
}

// Spill sorts and spills a new run of records to a file in the MergeSort's
// temp directory.  Since we sort each chunk in memory before spilling, the
// different chunks can be easily merged into sorted order when reading back
// the chunks sequentially.
func (r *MergeSort) Spill(ctx context.Context, vals []super.Value) error {
	var zr sio.Reader
	// Sorting can be slow, so check for cancellation.
	if err := goWithContext(ctx, func() {
		zr = r.comparator.SortStableReader(vals)
	}); err != nil {
		return err
	}
	filename := filepath.Join(r.tempDir, strconv.Itoa(r.nspill))
	runFile, err := newPeeker(ctx, r.sctx, filename, r.nspill, zr)
	if err != nil {
		return err
	}
	size, err := runFile.Size()
	if err != nil {
		return err
	}
	r.nspill++
	r.spillSize += size
	heap.Push(r, runFile)
	return nil
}

func goWithContext(ctx context.Context, f func()) error {
	ch := make(chan struct{})
	go func() {
		f()
		close(ch)
	}()
	select {
	case <-ch:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Peek returns the next record without advancing the reader.  The record stops
// being valid at the next read call.
func (r *MergeSort) Peek() (*super.Value, error) {
	if r.Len() == 0 {
		return nil, nil
	}
	return r.runs[0].nextRecord, nil
}

// Read returns the smallest record (per the comparison function provided to MewMergeSort)
// from among the next records in the spilled chunks.  It implements the merge operation
// for an external merge sort.
func (r *MergeSort) Read() (*super.Value, error) {
	for {
		if r.Len() == 0 {
			return nil, nil
		}
		rec, eof, err := r.runs[0].read()
		if err != nil {
			return nil, err
		}
		if eof {
			if err := r.runs[0].CloseAndRemove(); err != nil {
				return nil, err
			}
			heap.Pop(r)
		} else {
			heap.Fix(r, 0)
		}
		if rec != nil {
			return rec, nil
		}
	}
}

func (r *MergeSort) SpillSize() int64 {
	return r.spillSize
}

func (r *MergeSort) Len() int { return len(r.runs) }

func (r *MergeSort) Less(i, j int) bool {
	if v := r.comparator.Compare(*r.runs[i].nextRecord, *r.runs[j].nextRecord); v != 0 {
		return v < 0
	}
	// Maintain stability.
	return r.runs[i].ordinal < r.runs[j].ordinal
}

func (r *MergeSort) Swap(i, j int) { r.runs[i], r.runs[j] = r.runs[j], r.runs[i] }

func (r *MergeSort) Push(x any) { r.runs = append(r.runs, x.(*peeker)) }

func (r *MergeSort) Pop() any {
	x := r.runs[len(r.runs)-1]
	r.runs = r.runs[:len(r.runs)-1]
	return x
}
