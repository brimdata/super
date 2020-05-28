package archive

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/brimsec/zq/pkg/nano"
)

type Visitor func(zardir string) error

func ZarDirToLog(path string) string {
	return strings.TrimSuffix(path, zarExt)
}

func LogToZarDir(path string) string {
	return path + zarExt
}

// Localize maps the provided relative path name into absolute path
// names relative to the given zardir and returns the result.  The
// special name "_" is mapped to the path of the log file that
// corresponds to this zardir.
func Localize(zardir string, pathname string) string {
	if pathname == "_" {
		return ZarDirToLog(zardir)
	}
	return filepath.Join(zardir, pathname)
}

// Walk traverses the archive invoking the visitor on the zar dir corresponding
// to each log file, creating the zar dir if needed.
func Walk(ark *Archive, visit Visitor) error {
	for _, s := range ark.Meta.Spans {
		zardir := LogToZarDir(filepath.Join(ark.Root, s.Part))
		if err := os.MkdirAll(zardir, 0700); err != nil {
			return err
		}
		if err := visit(zardir); err != nil {
			return err
		}
	}
	return nil
}

type SpanVisitor func(span nano.Span, zngpath string) error

func SpanWalk(ark *Archive, v SpanVisitor) error {
	for _, s := range ark.Meta.Spans {
		if err := v(s.Span, filepath.Join(ark.Root, s.Part)); err != nil {
			return err
		}
	}
	return nil
}

// RmDirs descends a directory hierarchy looking for zar dirs and remove
// each such directory and all of its contents.
func RmDirs(ark *Archive) error {
	return Walk(ark, os.RemoveAll)
}
