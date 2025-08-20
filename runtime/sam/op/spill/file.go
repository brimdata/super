package spill

import (
	"bufio"
	"os"

	"github.com/brimdata/super"
	"github.com/brimdata/super/pkg/bufwriter"
	"github.com/brimdata/super/pkg/fs"
	"github.com/brimdata/super/sio"
	"github.com/brimdata/super/sio/bsupio"
)

// File provides a means to write a sequence of Super values to temporary
// storage then read them back.  This is used for processing large batches of
// data that do not fit in memory and/or cannot be shuffled to a peer worker,
// but can be processed in multiple passes.  File implements sio.Reader and
// sio.Writer.
type File struct {
	*bsupio.Reader
	*bsupio.Writer
	file *os.File
}

// NewFile returns a File.  Records should be written to File via the sio.Writer
// interface, followed by a call to the Rewind method, followed by reading
// records via the sio.Reader interface.
func NewFile(f *os.File) *File {
	return &File{
		Writer: bsupio.NewWriterWithOpts(bufwriter.New(sio.NopCloser(f)), bsupio.WriterOpts{
			Compress:    false, // Compression reduces write throughput; see #3973.
			FrameThresh: bsupio.DefaultFrameThresh,
		}),
		file: f,
	}
}

func NewTempFile() (*File, error) {
	f, err := TempFile()
	if err != nil {
		return nil, err
	}
	return NewFile(f), nil
}

func NewFileWithPath(path string) (*File, error) {
	f, err := fs.Create(path)
	if err != nil {
		return nil, err
	}
	return NewFile(f), nil
}

func (f *File) Rewind(sctx *super.Context) error {
	// Close the writer to flush any pending output but since we
	// wrapped the file in a sio.NopCloser, the file will stay open.
	if err := f.Writer.Close(); err != nil {
		return err
	}
	f.Writer = nil
	if _, err := f.file.Seek(0, 0); err != nil {
		return err
	}
	if f.Reader != nil {
		f.Reader.Close()
	}
	f.Reader = bsupio.NewReader(sctx, bufio.NewReader(f.file))
	return nil
}

// CloseAndRemove closes and removes the underlying file.
func (r *File) CloseAndRemove() error {
	if r.Reader != nil {
		r.Reader.Close()
	}
	err := r.file.Close()
	if rmErr := os.Remove(r.file.Name()); err == nil {
		err = rmErr
	}
	return err
}

func (f *File) Size() (int64, error) {
	info, err := f.file.Stat()
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}
