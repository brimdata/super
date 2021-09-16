package main

import "C"

import (
	"context"
	"errors"

	"github.com/brimdata/zed/compiler"
	"github.com/brimdata/zed/driver"
	"github.com/brimdata/zed/pkg/storage"
	"github.com/brimdata/zed/zio/anyio"
	"github.com/brimdata/zed/zio/emitter"
	"github.com/brimdata/zed/zson"
)

// result converts an error into response structure expected
// by the Python calling code. cgo does not support exporting
// a function that returns a struct, hence the multiple return
// values.
// If C.CString is used to allocate a C char* string, the Python
// side code will free it.
func result(err error) (*C.char, bool) {
	if err != nil {
		return C.CString(err.Error()), false
	}
	return nil, true
}

// ErrorTest is only used to verify that errors are successfully passed
// between the Go & Python realms.
//
//export ErrorTest
func ErrorTest() (*C.char, bool) {
	return result(errors.New("error test"))
}

//export ZedFileEval
func ZedFileEval(inquery, inpath, informat, outpath, outformat string) (*C.char, bool) {
	return result(doZedFileEval(inquery, inpath, informat, outpath, outformat))
}

func doZedFileEval(inquery, inpath, informat, outpath, outformat string) (err error) {
	if inpath == "-" {
		inpath = "/dev/stdin"
	}
	if outpath == "-" {
		outpath = "/dev/stdout"
	}
	query, err := compiler.ParseProc(inquery)
	if err != nil {
		return err
	}

	zctx := zson.NewContext()
	local := storage.NewLocalEngine()
	rc, err := anyio.OpenFile(zctx, local, inpath, anyio.ReaderOpts{
		Format: informat,
	})
	if err != nil {
		return err
	}
	defer rc.Close()

	w, err := emitter.NewFileFromPath(context.Background(), local, outpath, anyio.WriterOpts{
		Format: outformat,
	})
	if err != nil {
		return err
	}
	defer func() {
		closeErr := w.Close()
		if err == nil {
			err = closeErr
		}
	}()

	d := driver.NewCLI(w)
	return driver.RunWithReader(context.Background(), d, query, zctx, rc, nil)
}

func main() {}
