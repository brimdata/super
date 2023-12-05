package vngio

import (
	"errors"
	"io"
	"os"

	"github.com/brimdata/zed"
	"github.com/brimdata/zed/compiler/optimizer/demand"
	"github.com/brimdata/zed/vector"
	"github.com/brimdata/zed/vng"
	"github.com/brimdata/zed/zio"
)

func NewReader(zctx *zed.Context, r io.Reader, demandOut demand.Demand) (zio.Reader, error) {
	s, ok := r.(io.Seeker)
	if !ok {
		return nil, errors.New("VNG must be used with a seekable input")
	}
	ra, ok := r.(io.ReaderAt)
	if !ok {
		return nil, errors.New("VNG must be used with an io.ReaderAt")
	}
	size, err := s.Seek(0, io.SeekEnd)
	if err != nil {
		return nil, err
	}
	o, err := vng.NewObject(zctx, ra, size)
	if err != nil {
		return nil, err
	}
	if os.Getenv("ZED_USE_VECTOR") != "" {
		if demandOut == nil {
			demandOut = demand.All()
		}
		return vector.NewReader(o, demandOut), nil
	} else {
		return vng.NewReader(o)
	}
}
