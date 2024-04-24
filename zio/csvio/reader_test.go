package csvio

import (
	"runtime"
	"strings"
	"testing"

	"github.com/brimdata/zed"
	"github.com/stretchr/testify/require"
)

func TestNewReaderUsesContextParameter(t *testing.T) {
	zctx := zed.NewContext()
	r := NewReader(zctx, strings.NewReader("f\n1\n"), ReaderOpts{})
	defer runtime.KeepAlive(r)
	rec, err := r.Read()
	require.NoError(t, err)
	typ, err := zctx.LookupType(rec.Type().ID())
	require.NoError(t, err)
	require.Exactly(t, rec.Type(), typ)
}
