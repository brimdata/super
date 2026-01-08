package queryflags

import (
	"flag"
	"fmt"
	"os"

	"github.com/brimdata/super/compiler/srcfiles"
	"github.com/brimdata/super/sbuf"
	"github.com/brimdata/super/sup"
)

type QueryTextFlags struct {
	Query     []srcfiles.Input
	includes  FileInput
	dashCArgs PlainInput
}

type Flags struct {
	Stats bool
	QueryTextFlags
}

func (q *QueryTextFlags) SetFlags(fs *flag.FlagSet) {
	q.includes.inputs = &q.Query
	q.dashCArgs.inputs = &q.Query
	fs.Var(&q.dashCArgs, "c", "query text (may be used multiple times)")
	fs.Var(&q.includes, "I", "source file containing query text (may be used multiple times)")
}

func (f *Flags) SetFlags(fs *flag.FlagSet) {
	fs.BoolVar(&f.Stats, "stats", false, "display search stats on stderr")
	f.QueryTextFlags.SetFlags(fs)
}

func (f *Flags) PrintStats(stats sbuf.Progress) {
	if f.Stats {
		out, err := sup.Marshal(stats)
		if err != nil {
			out = fmt.Sprintf("error marshaling stats: %s", err)
		}
		fmt.Fprintln(os.Stderr, out)
	}
}

type PlainInput struct {
	inputs *[]srcfiles.Input
}

func (p *PlainInput) Set(value string) error {
	*p.inputs = append(*p.inputs, &srcfiles.PlainInput{Text: value})
	return nil
}

func (PlainInput) String() string {
	return ""
}

type FileInput struct {
	inputs *[]srcfiles.Input
}

func (f *FileInput) Set(value string) error {
	*f.inputs = append(*f.inputs, &srcfiles.FileInput{Name: value})
	return nil
}

func (FileInput) String() string {
	return ""
}
