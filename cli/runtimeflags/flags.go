package runtimeflags

import (
	"errors"
	"flag"

	"github.com/brimdata/super/cli/auto"
	"github.com/brimdata/super/runtime/sam/expr/agg"
	"github.com/brimdata/super/runtime/sam/op/sort"
	"github.com/pbnjay/memory"
)

// defaultMemMaxBytes returns approximately 1/8 of total system memory,
// in bytes, bounded between 128MiB and 1GiB.
func defaultMemMaxBytes() uint64 {
	tm := memory.TotalMemory()
	const gig = 1024 * 1024 * 1024
	switch {
	case tm <= 1*gig:
		return 128 * 1024 * 1024
	case tm <= 2*gig:
		return 256 * 1024 * 1024
	case tm <= 4*gig:
		return 512 * 1024 * 1024
	default:
		return 1 * gig
	}
}

type Flags struct {
	// these memory limits should be based on a shared resource model
	aggMemMax  auto.Bytes
	sortMemMax auto.Bytes
}

func (e *Flags) SetFlags(fs *flag.FlagSet) {
	e.aggMemMax = auto.NewBytes(uint64(agg.MaxValueSize))
	fs.Var(&e.aggMemMax, "aggmem", "maximum memory used per aggregate function value in MiB, MB, etc")
	def := defaultMemMaxBytes()
	e.sortMemMax = auto.NewBytes(def)
	fs.Var(&e.sortMemMax, "sortmem", "maximum memory used by sort in MiB, MB, etc")
}

func (e *Flags) Init() error {
	if e.aggMemMax.Bytes <= 0 {
		return errors.New("aggmem value must be greater than zero")
	}
	agg.MaxValueSize = int(e.aggMemMax.Bytes)
	if e.sortMemMax.Bytes <= 0 {
		return errors.New("sortmem value must be greater than zero")
	}
	sort.MemMaxBytes = int(e.sortMemMax.Bytes)
	return nil
}
