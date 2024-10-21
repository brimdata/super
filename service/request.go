package service

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/brimdata/super"
	"github.com/brimdata/super/api"
	"github.com/brimdata/super/compiler/optimizer/demand"
	"github.com/brimdata/super/compiler/parser"
	"github.com/brimdata/super/lake"
	"github.com/brimdata/super/lake/branches"
	"github.com/brimdata/super/lake/commits"
	"github.com/brimdata/super/lake/journal"
	"github.com/brimdata/super/lake/pools"
	"github.com/brimdata/super/lakeparse"
	"github.com/brimdata/super/service/srverr"
	"github.com/brimdata/super/zio"
	"github.com/brimdata/super/zio/anyio"
	"github.com/brimdata/super/zson"
	"github.com/gorilla/mux"
	"github.com/segmentio/ksuid"
	"go.uber.org/zap"
)

type Request struct {
	*http.Request
	Logger *zap.Logger
}

func newRequest(w http.ResponseWriter, r *http.Request, c *Core) (*ResponseWriter, *Request, bool) {
	req := &Request{Request: r}
	req.Logger = c.logger.With(zap.String("request_id", req.ID()))
	m := zson.NewZNGMarshaler()
	m.Decorate(zson.StylePackage)
	res := &ResponseWriter{
		ResponseWriter: w,
		Logger:         req.Logger,
		marshaler:      m,
		request:        req,
	}
	ss := strings.Split(r.Header.Get("Accept"), ",")
	if len(ss) == 0 {
		ss = []string{""}
	}
	for _, mime := range ss {
		format, err := api.MediaTypeToFormat(mime, c.conf.DefaultResponseFormat)
		if err != nil {
			continue
		}
		res.Format = format
		return res, req, true
	}
	res.Error(srverr.ErrInvalid("could not find supported MIME type in Accept header"))
	return nil, nil, false
}

func (r *Request) openPool(w *ResponseWriter, root *lake.Root) (*lake.Pool, bool) {
	id, ok := r.PoolID(w, root)
	if !ok {
		return nil, false
	}
	pool, err := root.OpenPool(r.Context(), id)
	if err != nil {
		w.Error(err)
		return nil, false
	}
	return pool, true
}

func (r *Request) ID() string {
	return api.RequestIDFromContext(r.Context())
}

func (r *Request) PoolID(w *ResponseWriter, root *lake.Root) (ksuid.KSUID, bool) {
	s, ok := r.StringFromPath(w, "pool")
	if !ok {
		return ksuid.Nil, false
	}
	if id, err := lakeparse.ParseID(s); err == nil {
		if _, err = root.OpenPool(r.Context(), id); err == nil {
			return id, true
		}
	}
	id, err := root.PoolID(r.Context(), s)
	if errors.Is(err, pools.ErrNotFound) {
		w.Error(err)
		return ksuid.Nil, false
	}
	if err != nil {
		w.Error(srverr.ErrInvalid("invalid path param %q: %w", s, err))
		return ksuid.Nil, false
	}
	return id, true
}

func (r *Request) CommitID(w *ResponseWriter) (ksuid.KSUID, bool) {
	return r.TagFromPath(w, "commit")
}

func (r *Request) decodeCommitMessage(w *ResponseWriter) (api.CommitMessage, bool) {
	commitJSON := r.Header.Get("Zed-Commit")
	var message api.CommitMessage
	if commitJSON != "" {
		if err := json.Unmarshal([]byte(commitJSON), &message); err != nil {
			w.Error(srverr.ErrInvalid("load endpoint encountered invalid JSON in Zed-Commit header: %w", err))
			return message, false
		}
	}
	return message, true
}

func (r *Request) StringFromPath(w *ResponseWriter, arg string) (string, bool) {
	v := mux.Vars(r.Request)
	s, ok := v[arg]
	if !ok {
		w.Error(srverr.ErrInvalid("no arg %q in path", arg))
		return "", false
	}
	decoded, err := url.QueryUnescape(s)
	return decoded, err == nil
}

func (r *Request) TagFromPath(w *ResponseWriter, arg string) (ksuid.KSUID, bool) {
	v := mux.Vars(r.Request)
	s, ok := v[arg]
	if !ok {
		w.Error(srverr.ErrInvalid("no arg %q in path", arg))
		return ksuid.Nil, false
	}
	id, err := lakeparse.ParseID(s)
	if err != nil {
		w.Error(srverr.ErrInvalid("invalid path param %q: %w", arg, err))
		return ksuid.Nil, false
	}
	return id, true
}

func (r *Request) JournalIDFromQuery(w *ResponseWriter, param string) (journal.ID, bool) {
	s := r.URL.Query().Get(param)
	if s == "" {
		return journal.Nil, true
	}
	id, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		w.Error(srverr.ErrInvalid("invalid query param %q: %w", param, err))
		return journal.Nil, false
	}
	return journal.ID(id), true
}

func (r *Request) BoolFromQuery(w *ResponseWriter, param string) (bool, bool) {
	s := r.URL.Query().Get(param)
	if s == "" {
		return false, true
	}
	b, err := strconv.ParseBool(s)
	if err != nil {
		w.Error(srverr.ErrInvalid("invalid query param %q: %w", s, err))
		return false, false
	}
	return b, true
}

func (r *Request) Unmarshal(w *ResponseWriter, body interface{}, templates ...interface{}) bool {
	format, ok := r.format(w, DefaultZedFormat)
	if !ok {
		return false
	}
	zrc, err := anyio.NewReaderWithOpts(super.NewContext(), r.Body, demand.All(), anyio.ReaderOpts{Format: format})
	if err != nil {
		w.Error(srverr.ErrInvalid(err))
		return false
	}
	defer zrc.Close()
	zv, err := zrc.Read()
	if err != nil {
		w.Error(srverr.ErrInvalid(err))
		return false
	}
	if zv == nil {
		return true
	}
	m := zson.NewZNGUnmarshaler()
	m.Bind(templates...)
	if err := m.Unmarshal(*zv, body); err != nil {
		w.Error(srverr.ErrInvalid(err))
		return false
	}
	return true
}

func (r *Request) format(w *ResponseWriter, dflt string) (string, bool) {
	format, err := api.MediaTypeToFormat(r.Header.Get("Content-Type"), dflt)
	if err != nil {
		var uerr *api.ErrUnsupportedMimeType
		if errors.As(err, &uerr) && uerr.Type == "application/x-www-form-urlencoded" {
			// curl will by default set the Accept header to
			// application/x-www-from-urlencoded so assume the
			// default format if this is the case.
			return dflt, true
		}
		w.Error(srverr.ErrInvalid(err))
		return "", false
	}
	return format, true
}

type ResponseWriter struct {
	http.ResponseWriter
	Format    string
	Logger    *zap.Logger
	zw        zio.WriteCloser
	marshaler *zson.MarshalZNGContext
	request   *Request
	written   int32
}

func (w *ResponseWriter) ContentType() string {
	return w.Header().Get("Content-Type")
}

func (w *ResponseWriter) ZioWriter() zio.WriteCloser {
	if w.zw == nil {
		typ, err := api.FormatToMediaType(w.Format)
		if err != nil {
			w.Error(err)
			return nil
		}
		w.Header().Set("Content-Type", typ)
		w.zw, err = anyio.NewWriter(zio.NopCloser(w), anyio.WriterOpts{Format: w.Format})
		if err != nil {
			w.Error(err)
			return nil
		}
	}
	return w.zw
}

func (w *ResponseWriter) Write(b []byte) (int, error) {
	if atomic.CompareAndSwapInt32(&w.written, 0, 1) {
		typ, err := api.FormatToMediaType(w.Format)
		if err != nil {
			return 0, err
		}
		w.Header().Set("Content-Type", typ)
	}
	return w.ResponseWriter.Write(b)
}

func (w *ResponseWriter) Respond(status int, body interface{}) bool {
	w.WriteHeader(status)
	return w.Marshal(body)
}

func (w *ResponseWriter) Error(err error) {
	if err == context.Canceled && err == w.request.Context().Err() {
		w.Logger.Info("Request context canceled")
		return
	}
	status, res := errorResponse(err)
	if status >= 500 {
		w.Logger.Warn("Error", zap.Int("status", status), zap.Error(err))
	}
	if atomic.CompareAndSwapInt32(&w.written, 0, 1) {
		// Should errors be returned in different encodings, i.e. adhere to
		// the encoding ?
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		if err := json.NewEncoder(w).Encode(res); err != nil {
			w.Logger.Warn("Error writing response", zap.Error(err))
		}
	}
}

func (w *ResponseWriter) Marshal(body interface{}) bool {
	rec, err := w.marshaler.Marshal(body)
	if err != nil {
		// XXX If status header has not been sent this should send error.
		w.Error(err)
		return false
	}
	zw := w.ZioWriter()
	if zw == nil {
		return false
	}
	if err := zw.Write(rec); err != nil {
		w.Error(err)
		return false
	}
	zw.Close()
	return true
}

func errorResponse(e error) (status int, ae *api.Error) {
	status = http.StatusInternalServerError
	ae = &api.Error{Type: "Error"}

	if list := (parser.ErrorList)(nil); errors.As(e, &list) {
		ae.CompilationErrors = list
	}

	var ze *srverr.Error
	if !errors.As(e, &ze) {
		ze = &srverr.Error{Err: e}
	}

	switch {
	case errors.Is(e, branches.ErrExists) || errors.Is(e, pools.ErrExists):
		ze.Kind = srverr.Conflict
	case errors.Is(e, branches.ErrNotFound) || errors.Is(e, commits.ErrNotFound) ||
		errors.Is(e, pools.ErrNotFound) || errors.Is(e, fs.ErrNotExist):
		ze.Kind = srverr.NotFound
	}

	switch ze.Kind {
	case srverr.Invalid:
		status = http.StatusBadRequest
	case srverr.NotFound:
		status = http.StatusNotFound
	case srverr.Exists:
		status = http.StatusBadRequest
	case srverr.Conflict:
		status = http.StatusConflict
	case srverr.NoCredentials:
		status = http.StatusUnauthorized
	case srverr.Forbidden:
		status = http.StatusForbidden
	}

	ae.Kind = ze.Kind.String()
	ae.Message = ze.Message()
	return
}
