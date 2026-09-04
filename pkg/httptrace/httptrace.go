// Package httptrace provides an opt-in log of the HTTP requests issued by the
// from operator.  It is enabled by the SUPER_HTTP_TRACE environment variable:
// "-" or "stderr" logs to stderr and any other value is a file path opened for
// append.
//
// This is a debugging aid, not a fix.  A query that reads from an API has no
// way to see the status code of the responses it is reading (issue #7096), so
// an authentication failure or a rate limit arrives as ordinary data and a
// paged walk can terminate early and silently.  The trace makes those statuses
// visible on a side channel without changing what the query returns.
package httptrace

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"
)

var dst = func() io.Writer {
	switch path := os.Getenv("SUPER_HTTP_TRACE"); path {
	case "":
		return nil
	case "-", "stderr":
		return os.Stderr
	default:
		f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			fmt.Fprintf(os.Stderr, "SUPER_HTTP_TRACE: %s\n", err)
			return os.Stderr
		}
		return f
	}
}()

var mu sync.Mutex

// Enabled reports whether tracing is on so a caller can skip any setup work.
func Enabled() bool { return dst != nil }

func logf(format string, args ...any) {
	mu.Lock()
	defer mu.Unlock()
	fmt.Fprintf(dst, "%s %s\n", time.Now().Format(time.RFC3339Nano), fmt.Sprintf(format, args...))
}

// Do performs req with client, logging the method, URL, and either the response
// status or the transport error.  It is a drop-in replacement for client.Do.
func Do(client *http.Client, req *http.Request) (*http.Response, error) {
	if !Enabled() {
		return client.Do(req)
	}
	start := time.Now()
	resp, err := client.Do(req)
	elapsed := time.Since(start).Round(time.Millisecond)
	if err != nil {
		logf("%s %s -> transport error after %s: %s", req.Method, req.URL, elapsed, err)
		return resp, err
	}
	logf("%s %s -> %s (%s)", req.Method, req.URL, resp.Status, elapsed)
	return resp, err
}
