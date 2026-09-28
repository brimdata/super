package jsonio

import (
	"bytes"
	"strings"
	"testing"

	"github.com/brimdata/super"
	"github.com/brimdata/super/sio"
	"github.com/brimdata/super/sup"
)

func TestNestedNoneJSON(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"none", ""},
		{"[none]", "[null]\n"},
		{"{a:[none]}", `{"a":[null]}` + "\n"},
		{"error(none)", `{"error":null}` + "\n"},
		{"{a:1,b?:none::int64}", `{"a":1}` + "\n"},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			val, err := sup.ParseValue(super.NewContext(), tc.in)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			var buf bytes.Buffer
			w := NewWriter(sio.NopCloser(&buf), WriterOpts{})
			if err := w.Write(val); err != nil {
				t.Fatalf("write: %v", err)
			}
			if err := w.Close(); err != nil {
				t.Fatalf("close: %v", err)
			}
			got := buf.String()
			if got != tc.want {
				t.Fatalf("got %q want %q", got, tc.want)
			}
		})
	}
}

func TestNestedNoneNoPanic(t *testing.T) {
	for _, in := range []string{"[none]", "{a:[none]}", "error(none)", "values none | values len(this)"} {
		// last case is spq; skip — covered by CLI. Keep value-level only.
		if strings.Contains(in, "|") {
			continue
		}
		val, err := sup.ParseValue(super.NewContext(), in)
		if err != nil {
			t.Fatalf("parse %s: %v", in, err)
		}
		var buf bytes.Buffer
		w := NewWriter(sio.NopCloser(&buf), WriterOpts{})
		if err := w.Write(val); err != nil {
			t.Fatalf("write %s: %v", in, err)
		}
		_ = w.Close()
	}
}
