package httpd_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/brimdata/super/pkg/httpd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRequestContextClosure(t *testing.T) {
	errCh := make(chan error)
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		ctx := r.Context()
		<-ctx.Done()
		errCh <- ctx.Err()
	})
	srv := httpd.New("127.0.0.1:", h)
	ctx, cancel := context.WithCancel(t.Context())
	require.NoError(t, srv.Start(ctx))
	res, err := http.Get(fmt.Sprintf("http://%s/", srv.Addr()))
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, res.StatusCode)
	cancel()
	select {
	case err = <-errCh:
	case <-time.After(time.Second):
		t.Fatalf("context did not cancel after one second")
	}
	assert.Equal(t, context.Canceled, err)
	assert.NoError(t, srv.Wait())
}

func TestDeadlineExceeded(t *testing.T) {
	old := httpd.ShutdownTimeout
	httpd.ShutdownTimeout = 1
	defer func() { httpd.ShutdownTimeout = old }()
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		time.Sleep(time.Second * 5) // Essentially sleep forever.

	})
	srv := httpd.New("127.0.0.1:", h)
	ctx, cancel := context.WithCancel(t.Context())
	require.NoError(t, srv.Start(ctx))
	_, err := http.Get(fmt.Sprintf("http://%s/", srv.Addr()))
	require.NoError(t, err)
	cancel()
	require.Equal(t, context.DeadlineExceeded, srv.Wait())
}
