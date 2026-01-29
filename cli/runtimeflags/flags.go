package runtimeflags

import (
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/brimdata/super/cli/auto"
	"github.com/brimdata/super/runtime/exec"
	"github.com/brimdata/super/runtime/sam/expr/agg"
	"github.com/brimdata/super/runtime/sam/op/fuse"
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

type EngineFlags struct {
	sam     bool
	vam     bool
	Runtime exec.Runtime
}

type Flags struct {
	// these memory limits should be based on a shared resource model
	aggMemMax  auto.Bytes
	sortMemMax auto.Bytes
	fuseMemMax auto.Bytes
	EngineFlags
}

func (e *EngineFlags) SetFlags(fs *flag.FlagSet) {
	fs.BoolVar(&e.sam, "sam", false, "execute query in sequential runtime")
	fs.BoolVar(&e.vam, "vam", false, "execute query in vector runtime")
}

func (e *Flags) SetFlags(fs *flag.FlagSet) {
	e.aggMemMax = auto.NewBytes(uint64(agg.MaxValueSize))
	fs.Var(&e.aggMemMax, "aggmem", "maximum memory used per aggregate function value in MiB, MB, etc")
	def := defaultMemMaxBytes()
	e.sortMemMax = auto.NewBytes(def)
	fs.Var(&e.sortMemMax, "sortmem", "maximum memory used by sort in MiB, MB, etc")
	e.fuseMemMax = auto.NewBytes(def)
	fs.Var(&e.fuseMemMax, "fusemem", "maximum memory used by fuse in MiB, MB, etc")
	e.EngineFlags.SetFlags(fs)
}

func (e *EngineFlags) Init() error {
	var err error
	e.Runtime, err = e.getRuntime()
	return err
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
	if e.fuseMemMax.Bytes <= 0 {
		return errors.New("fusemem value must be greater than zero")
	}
	fuse.MemMaxBytes = int(e.fuseMemMax.Bytes)
	if e.sam && e.vam {
		return errors.New("sam and vam flags cannot both be enabled")
	}
	return e.EngineFlags.Init()
}

func (e *EngineFlags) getRuntime() (exec.Runtime, error) {
	// Flags take precedence.
	if e.sam {
		return exec.RuntimeSAM, nil
	}
	if e.vam {
		return exec.RuntimeVAM, nil
	}
	// Then environment variable.
	if rt := os.Getenv("SUPER_RUNTIME"); rt != "" {
		switch rt {
		case "sam":
			return exec.RuntimeSAM, nil
		case "vam":
			return exec.RuntimeVAM, nil
		default:
			return exec.RuntimeAuto, fmt.Errorf("invalid SUPER_RUNTIME value: %q (must be \"vam\" or \"sam\")", rt)
		}
	}
	return exec.RuntimeAuto, nil

}
