package main

import (
	"fmt"
	"os"

	_ "github.com/brimdata/super/cmd/super/compile"
	_ "github.com/brimdata/super/cmd/super/db/auth"
	_ "github.com/brimdata/super/cmd/super/db/branch"
	_ "github.com/brimdata/super/cmd/super/db/compact"
	_ "github.com/brimdata/super/cmd/super/db/compile"
	_ "github.com/brimdata/super/cmd/super/db/create"
	_ "github.com/brimdata/super/cmd/super/db/delete"
	_ "github.com/brimdata/super/cmd/super/db/drop"
	_ "github.com/brimdata/super/cmd/super/db/init"
	_ "github.com/brimdata/super/cmd/super/db/load"
	_ "github.com/brimdata/super/cmd/super/db/log"
	_ "github.com/brimdata/super/cmd/super/db/ls"
	_ "github.com/brimdata/super/cmd/super/db/manage"
	_ "github.com/brimdata/super/cmd/super/db/merge"
	_ "github.com/brimdata/super/cmd/super/db/query"
	_ "github.com/brimdata/super/cmd/super/db/rename"
	_ "github.com/brimdata/super/cmd/super/db/revert"
	_ "github.com/brimdata/super/cmd/super/db/serve"
	_ "github.com/brimdata/super/cmd/super/db/use"
	_ "github.com/brimdata/super/cmd/super/db/vacate"
	_ "github.com/brimdata/super/cmd/super/db/vacuum"
	_ "github.com/brimdata/super/cmd/super/db/vector"
	_ "github.com/brimdata/super/cmd/super/dev"
	_ "github.com/brimdata/super/cmd/super/dev/dig/frames"
	_ "github.com/brimdata/super/cmd/super/dev/dig/slice"
	_ "github.com/brimdata/super/cmd/super/dev/vector/copy"
	_ "github.com/brimdata/super/cmd/super/dev/vector/project"
	_ "github.com/brimdata/super/cmd/super/dev/vector/query"
	_ "github.com/brimdata/super/cmd/super/dev/vector/search"
	_ "github.com/brimdata/super/cmd/super/dev/vng"
	"github.com/brimdata/super/cmd/super/root"
)

func main() {
	if err := root.Super.Exec(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		os.Exit(1)
	}
}
