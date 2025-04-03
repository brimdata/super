package emitter

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/brimdata/super"
	"github.com/brimdata/super/pkg/storage"
	storagemock "github.com/brimdata/super/pkg/storage/mock"
	"github.com/brimdata/super/zio"
	"github.com/brimdata/super/zio/anyio"
	"github.com/brimdata/super/zio/supio"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestDirS3Source(t *testing.T) {
	path := "s3://testbucket/dir"
	const input = `
{_path:"conn",foo:"1"}
{_path:"http",bar:"2"}
`
	uri, err := storage.ParseURI(path)
	require.NoError(t, err)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	engine := storagemock.NewMockEngine(ctrl)

	engine.EXPECT().Put(context.Background(), uri.JoinPath("conn.sup")).
		Return(zio.NopCloser(bytes.NewBuffer(nil)), nil)
	engine.EXPECT().Put(context.Background(), uri.JoinPath("http.sup")).
		Return(zio.NopCloser(bytes.NewBuffer(nil)), nil)

	r := supio.NewReader(super.NewContext(), strings.NewReader(input))
	require.NoError(t, err)
	w, err := NewSplit(context.Background(), engine, uri, "", false, anyio.WriterOpts{Format: "sup"})
	require.NoError(t, err)
	require.NoError(t, zio.Copy(w, r))
}
