package main

import (
	"fmt"
	"os"

	"github.com/brimdata/super/cmd/zed/dev/compile"
	"github.com/brimdata/super/cmd/zed/root"
)

func main() {
	root.Zed.Add(compile.Cmd)
	args := append([]string{"compile"}, os.Args[1:]...)
	if err := root.Zed.ExecRoot(args); err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		os.Exit(1)
	}
}
