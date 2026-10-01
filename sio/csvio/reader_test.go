package csvio

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/superdb/super"
)

func TestNewReaderUsesContextParameter(t *testing.T) {
	sctx := super.NewContext()
	rec, err := NewReader(sctx, strings.NewReader("f\n1\n"), ReaderOpts{}).Read()
	require.NoError(t, err)
	typ, err := sctx.LookupType(rec.Type().ID())
	require.NoError(t, err)
	require.Exactly(t, rec.Type(), typ)
}
