package zbuf

import (
	"io"

	"github.com/brimdata/super/sio"
)

type File struct {
	sio.Reader
	c    io.Closer
	name string
}

func NewFile(r sio.Reader, c io.Closer, name string) *File {
	return &File{r, c, name}
}

func (r *File) Close() error {
	return r.c.Close()
}

func (r *File) String() string {
	return r.name
}
