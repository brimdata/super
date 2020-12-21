package ingest

import (
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"sync/atomic"

	"github.com/brimsec/zq/zbuf"
	"github.com/brimsec/zq/zio"
	"github.com/brimsec/zq/zio/detector"
	"github.com/brimsec/zq/zio/ndjsonio"
	"github.com/brimsec/zq/zio/zngio"
	"github.com/brimsec/zq/zng"
	"github.com/brimsec/zq/zng/resolver"
	"github.com/brimsec/zq/zqe"
)

type MultipartLogReader struct {
	mr        *multipart.Reader
	opts      zio.ReaderOpts
	stopOnErr bool
	warnings  []string
	zreader   *zbuf.File
	zctx      *resolver.Context
	nread     int64
}

func NewMultipartLogReader(mr *multipart.Reader, zctx *resolver.Context) *MultipartLogReader {
	return &MultipartLogReader{
		mr:   mr,
		opts: zio.ReaderOpts{Zng: zngio.ReaderOpts{Validate: true}},
		zctx: zctx,
	}
}

func (m *MultipartLogReader) SetStopOnError() {
	m.stopOnErr = true
}

func (m *MultipartLogReader) Read() (*zng.Record, error) {
read:
	if m.zreader == nil {
		zr, err := m.next()
		if zr == nil || err != nil {
			return nil, err
		}
		m.zreader = zr
	}
	rec, err := m.zreader.Read()
	if err != nil || rec == nil {
		zr := m.zreader
		m.zreader.Close()
		m.zreader = nil
		if err != nil {
			if m.stopOnErr {
				return nil, err
			}
			m.appendWarning(zr, err)
		}
		goto read
	}
	return rec, err
}

func (m *MultipartLogReader) next() (*zbuf.File, error) {
next:
	if m.mr == nil {
		return nil, nil
	}
	part, err := m.mr.NextPart()
	if err != nil {
		if err == io.EOF {
			m.mr, err = nil, nil
		}
		return nil, err
	}
	if part.FormName() == "json_config" {
		if err := json.NewDecoder(part).Decode(&m.opts.JSON.TypeConfig); err != nil {
			return nil, zqe.ErrInvalid("bad typing config: %v", err)
		}
		m.opts.JSON.PathRegexp = ndjsonio.DefaultPathRegexp
		goto next
	}

	name := part.FileName()
	counter := &mpcounter{part, &m.nread}
	zr, err := detector.OpenFromNamedReadCloser(m.zctx, counter, name, m.opts)
	if err != nil {
		part.Close()
		if m.stopOnErr {
			return nil, err
		}
		m.appendWarning(zr, err)
		goto next
	}
	return zr, err
}

func (m *MultipartLogReader) appendWarning(zr *zbuf.File, err error) {
	m.warnings = append(m.warnings, fmt.Sprintf("%s: %s", zr, err))
}

func (m *MultipartLogReader) Warnings() []string {
	return m.warnings
}

func (m *MultipartLogReader) BytesRead() int64 {
	return atomic.LoadInt64(&m.nread)
}

type mpcounter struct {
	*multipart.Part
	nread *int64
}

func (r *mpcounter) Read(b []byte) (int, error) {
	n, err := r.Part.Read(b)
	atomic.AddInt64(r.nread, int64(n))
	return n, err
}
