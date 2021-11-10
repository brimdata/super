package main

import (
	"fmt"
	"os"

	"github.com/brimdata/zed/cmd/zed/api"
	_ "github.com/brimdata/zed/cmd/zed/lake/auth"
	_ "github.com/brimdata/zed/cmd/zed/lake/branch"
	_ "github.com/brimdata/zed/cmd/zed/lake/create"
	_ "github.com/brimdata/zed/cmd/zed/lake/delete"
	_ "github.com/brimdata/zed/cmd/zed/lake/drop"
	_ "github.com/brimdata/zed/cmd/zed/lake/find"
	_ "github.com/brimdata/zed/cmd/zed/lake/index"
	_ "github.com/brimdata/zed/cmd/zed/lake/init"
	_ "github.com/brimdata/zed/cmd/zed/lake/load"
	_ "github.com/brimdata/zed/cmd/zed/lake/log"
	_ "github.com/brimdata/zed/cmd/zed/lake/ls"
	_ "github.com/brimdata/zed/cmd/zed/lake/merge"
	_ "github.com/brimdata/zed/cmd/zed/lake/query"
	_ "github.com/brimdata/zed/cmd/zed/lake/rename"
	_ "github.com/brimdata/zed/cmd/zed/lake/revert"
	_ "github.com/brimdata/zed/cmd/zed/lake/serve"
	_ "github.com/brimdata/zed/cmd/zed/lake/use"
	_ "github.com/brimdata/zed/cmd/zed/lake/vacate"
	"github.com/brimdata/zed/cmd/zed/root"
)

func main() {
	root.Zed.Add(api.Cmd)
	args := append([]string{"api"}, os.Args[1:]...)
	if err := root.Zed.ExecRoot(args); err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		os.Exit(1)
	}
}
