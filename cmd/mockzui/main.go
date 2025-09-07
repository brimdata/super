// mockzui is a command for testing purposes only. It is designed to simulate
// the exact way Zui launches then forks a separate super process. super must be
// in $PATH for this to work.

package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"os/exec"
)

func die(err error) {
	if err != nil {
		panic(err)
	}
}

var (
	pidfile  string
	portfile string
	dbroot   string
)

func init() {
	flag.StringVar(&portfile, "portfile", "", "location to write database service port")
	flag.StringVar(&pidfile, "pidfile", "", "location to write database service pid")
	flag.StringVar(&dbroot, "db", "", "path to database")
	flag.Parse()
}

func main() {
	r, _, err := os.Pipe()
	die(err)

	if portfile == "" {
		fmt.Fprintln(os.Stderr, "must provide -portfile arg")
		os.Exit(1)
	}
	if pidfile == "" {
		fmt.Fprintln(os.Stderr, "must provide -pidfile arg")
		os.Exit(1)
	}
	args := []string{
		"db",
		"serve",
		"-l=localhost:0",
		"-db=" + dbroot,
		"-log.level=warn",
		"-portfile=" + portfile,
		fmt.Sprintf("-brimfd=%d", r.Fd()),
	}
	stderr := bytes.NewBuffer(nil)
	cmd := exec.Command("super", args...)
	cmd.Stderr = stderr
	cmd.ExtraFiles = []*os.File{r}

	err = cmd.Start()
	die(err)
	pid := fmt.Sprintf("%d", cmd.Process.Pid)
	err = os.WriteFile(pidfile, []byte(pid), 0644)
	die(err)
	cmd.Wait()
}
