package queryflags

import (
	"flag"
	"fmt"
	"os"

	"github.com/brimdata/super/zbuf"
	"github.com/brimdata/super/zson"
)

type Flags struct {
	Verbose  bool
	Stats    bool
	Includes Includes
}

func (f *Flags) SetFlags(fs *flag.FlagSet) {
	fs.BoolVar(&f.Stats, "s", false, "display search stats on stderr")
	fs.Var(&f.Includes, "I", "source file containing Zed query text (may be used multiple times)")
}

func (f *Flags) PrintStats(stats zbuf.Progress) {
	if f.Stats {
		out, err := zson.Marshal(stats)
		if err != nil {
			out = fmt.Sprintf("error marshaling stats: %s", err)
		}
		fmt.Fprintln(os.Stderr, out)
	}
}
