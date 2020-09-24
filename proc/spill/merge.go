package spill

import (
	"container/heap"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"

	"github.com/brimsec/zq/expr"
	"github.com/brimsec/zq/zng"
	"github.com/brimsec/zq/zng/resolver"
)

// MergeSort manages "runs" (files of sorted zng records) that are spilled to
// disk a chunk at a time, then read back and merged in sorted order, effectively
// implementing an external merge sort.
type MergeSort struct {
	runs      []*peeker
	compareFn expr.CompareFn
	tempDir   string
	zctx      *resolver.Context
}

const TempPrefix = "zq-spill-"

func TempDir() (string, error) {
	return ioutil.TempDir("", TempPrefix)
}

func TempFile() (*os.File, error) {
	return ioutil.TempFile("", TempPrefix)
}

// NewMergeSort returns a MergeSort to implement external merge sorts of a large
// zng record stream.  It creates a temporary directory to hold the collection
// of spilled chunks.  Call Cleanup to remove it.
func NewMergeSort(compareFn expr.CompareFn) (*MergeSort, error) {
	tempDir, err := TempDir()
	if err != nil {
		return nil, err
	}
	return &MergeSort{
		compareFn: compareFn,
		tempDir:   tempDir,
		zctx:      resolver.NewContext(),
	}, nil
}

func (r *MergeSort) Cleanup() {
	for _, run := range r.runs {
		run.closeAndRemove()
	}
	os.RemoveAll(r.tempDir)
}

// Spill sorts and spills a new run of records to a file in the MergeSort's
// temp directory.  Since we sort each chunk in memory before spilling, the
// different chunks can be easily merged into sorted order when reading back
// the chunks sequentially.
func (r *MergeSort) Spill(recs []*zng.Record) error {
	expr.SortStable(recs, r.compareFn)
	index := len(r.runs)
	filename := filepath.Join(r.tempDir, strconv.Itoa(index))
	runFile, err := newPeeker(filename, index, recs, r.zctx)
	if err != nil {
		return err
	}
	heap.Push(r, runFile)
	return nil
}

// Peek returns the next record without advancing the reader.  The record stops
// being valid at the next read call.
func (r *MergeSort) Peek() (*zng.Record, error) {
	if r.Len() == 0 {
		return nil, nil
	}
	return r.runs[0].nextRecord, nil
}

// Read returns the smallest record (per the comparison function provided to MewMergeSort)
// from among the next records in the spilled chunks.  It implements the merge operation
// for an external merge sort.
func (r *MergeSort) Read() (*zng.Record, error) {
	for {
		if r.Len() == 0 {
			return nil, nil
		}
		rec, eof, err := r.runs[0].read()
		if err != nil {
			return nil, err
		}
		if eof {
			r.runs[0].closeAndRemove()
			heap.Pop(r)
		} else {
			heap.Fix(r, 0)
		}
		if rec != nil {
			return rec, nil
		}
	}
}

func (r *MergeSort) Len() int { return len(r.runs) }

func (r *MergeSort) Less(i, j int) bool {
	v := r.compareFn(r.runs[i].nextRecord, r.runs[j].nextRecord)
	switch {
	case v < 0:
		return true
	case v == 0:
		// Maintain stability.
		return r.runs[i].ordinal < r.runs[j].ordinal
	default:
		return false
	}
}

func (r *MergeSort) Swap(i, j int) { r.runs[i], r.runs[j] = r.runs[j], r.runs[i] }

func (r *MergeSort) Push(x interface{}) { r.runs = append(r.runs, x.(*peeker)) }

func (r *MergeSort) Pop() interface{} {
	x := r.runs[len(r.runs)-1]
	r.runs = r.runs[:len(r.runs)-1]
	return x
}
