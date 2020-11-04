package post

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/brimsec/zq/api/client"
	"github.com/brimsec/zq/cmd/zapi/cmd"
	"github.com/brimsec/zq/cmd/zapi/format"
	"github.com/brimsec/zq/pkg/display"
	"github.com/mccanne/charm"
)

var LogStream = &charm.Spec{
	Name:  "post",
	Usage: "post [options] path...",
	Short: "stream log data to a space",
	New:   NewLogStream,
}

func init() {
	cmd.CLI.Add(LogStream)
}

type LogStreamCommand struct {
	*cmd.Command
	spaceFlags spaceFlags
	logwriter  *client.MultipartWriter
	start      time.Time
}

func NewLogStream(parent charm.Command, fs *flag.FlagSet) (charm.Command, error) {
	c := &LogStreamCommand{Command: parent.(*cmd.Command)}
	c.spaceFlags.SetFlags(fs)
	c.spaceFlags.cmd = c.Command
	return c, nil
}

func (c *LogStreamCommand) Run(args []string) (err error) {
	if len(args) == 0 {
		return errors.New("path arg(s) required")
	}
	if err := c.Init(&c.spaceFlags); err != nil {
		return err
	}
	paths, err := abspaths(args)
	if err != nil {
		return err
	}
	c.logwriter, err = client.MultipartFileWriter(paths...)
	if err != nil {
		return err
	}
	var out io.Writer
	var dp *display.Display
	if !c.NoFancy {
		dp = display.New(c, time.Second)
		out = dp.Bypass()
		go dp.Run()
	} else {
		out = os.Stdout
	}
	id, err := c.SpaceID()
	if err != nil {
		return err
	}
	c.start = time.Now()
	conn := c.Connection()
	res, err := conn.LogPostWriter(c.Context(), id, nil, c.logwriter)
	if err != nil {
		if c.Context().Err() != nil {
			fmt.Println("post aborted")
			os.Exit(1)
		}
		return err
	}
	if res.Warnings != nil {
		for _, warning := range res.Warnings {
			fmt.Fprintf(out, "warning: %s\n", warning)
		}
	}
	fmt.Fprintf(out, "posted %s in %v\n", format.Bytes(c.logwriter.BytesRead()), time.Since(c.start))
	return nil
}

func (c *LogStreamCommand) Display(w io.Writer) bool {
	total := c.logwriter.BytesTotal
	if total == 0 {
		io.WriteString(w, "posting...\n")
		return true
	}
	read := c.logwriter.BytesRead()
	percent := float64(read) / float64(total) * 100
	fmt.Fprintf(w, "%5.1f%% %s/%s\n", percent, format.Bytes(read), format.Bytes(total))
	return true
}
