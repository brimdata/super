package main

import (
	"fmt"
	"os"

	_ "github.com/brimdata/zq/ppl/cmd/zar/compact"
	_ "github.com/brimdata/zq/ppl/cmd/zar/find"
	_ "github.com/brimdata/zq/ppl/cmd/zar/import"
	_ "github.com/brimdata/zq/ppl/cmd/zar/index"
	_ "github.com/brimdata/zq/ppl/cmd/zar/ls"
	_ "github.com/brimdata/zq/ppl/cmd/zar/map"
	_ "github.com/brimdata/zq/ppl/cmd/zar/rm"
	_ "github.com/brimdata/zq/ppl/cmd/zar/rmdirs"
	"github.com/brimdata/zq/ppl/cmd/zar/root"
	_ "github.com/brimdata/zq/ppl/cmd/zar/stat"
	_ "github.com/brimdata/zq/ppl/cmd/zar/zq"
)

func main() {
	if _, err := root.Zar.ExecRoot(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		os.Exit(1)
	}
}
