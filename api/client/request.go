package client

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptrace"
	"time"

	"github.com/brimdata/zed/api"
	"github.com/brimdata/zed/zio"
	"github.com/brimdata/zed/zio/zngio"
	"github.com/brimdata/zed/zson"
)

type Request struct {
	Header  http.Header
	Method  string
	Path    string
	Body    interface{}
	RawBody io.Reader

	host string
	ctx  context.Context

	// trace fields
	dnsStartTime  time.Time
	firstByteTime time.Time
	getConnTime   time.Time
	gotConnInfo   httptrace.GotConnInfo
}

func newRequest(ctx context.Context, host string, h http.Header) *Request {
	if requestID := api.RequestIDFromContext(ctx); requestID != "" {
		h.Set(api.RequestIDHeader, requestID)
	}
	req := &Request{
		Header: h,
		host:   host,
	}
	// use trace to track timing
	req.ctx = httptrace.WithClientTrace(ctx, &httptrace.ClientTrace{
		DNSStart:             func(httptrace.DNSStartInfo) { req.dnsStartTime = time.Now() },
		GotConn:              func(g httptrace.GotConnInfo) { req.gotConnInfo = g },
		GetConn:              func(string) { req.getConnTime = time.Now() },
		GotFirstResponseByte: func() { req.firstByteTime = time.Now() },
	})
	return req
}

func (r *Request) HTTPRequest() (*http.Request, error) {
	r.Header.Set("Content-Type", api.MediaTypeZNG)
	r.Header.Set("Accept", api.MediaTypeZNG)
	body, err := r.reader()
	if err != nil {
		return nil, err
	}
	u := r.host + r.Path
	req, err := http.NewRequestWithContext(r.ctx, r.Method, u, body)
	if err != nil {
		return nil, err
	}
	req.Header = r.Header
	return req, nil
}

func (r *Request) reader() (io.Reader, error) {
	if b, ok := r.Body.(io.Reader); ok {
		return b, nil
	}
	var buf bytes.Buffer
	m := zson.NewZNGMarshaler()
	m.Decorate(zson.StylePackage)
	v, err := m.Marshal(r.Body)
	if err != nil {
		return nil, err
	}
	zw := zngio.NewWriter(zio.NopCloser(&buf), zngio.WriterOpts{})
	if err := zw.Write(&v); err != nil {
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return &buf, nil
}

func (r *Request) Duration() time.Duration {
	if r.gotConnInfo.Reused {
		return r.firstByteTime.Sub(r.getConnTime)
	}
	return r.firstByteTime.Sub(r.dnsStartTime)
}
