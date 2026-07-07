package csup_test

import (
	"bytes"
	"testing"

	"github.com/brimdata/super"
	"github.com/brimdata/super/csup"
	"github.com/brimdata/super/pkg/field"
	"github.com/brimdata/super/sio"
	"github.com/brimdata/super/sup"
	"github.com/brimdata/super/vector"
	"github.com/stretchr/testify/require"
)

func TestObjectProjectMetadata(t *testing.T) {
	var b bytes.Buffer
	w := csup.NewSerializer(sio.NopCloser(&b))
	sctx := super.NewContext()
	supValues := []string{
		"{a:1,b:{c:4,d:0.7}}",
		"{a:2,b:{c:5,d:0.8}}",
		"{a:3,b:{c:6,d:0.9}}",
	}
	builder := vector.NewDynamicValueBuilder()
	for _, s := range supValues {
		builder.Write(sup.MustParseValue(sctx, s))
	}
	require.NoError(t, w.Push(builder.Build(sctx)))
	require.NoError(t, w.Close())
	csupBytes := b.Bytes()

	o, err := csup.NewObject(bytes.NewReader(csupBytes))
	require.NoError(t, err)
	p := field.NewProjection(field.DottedList("b.d,a"))
	values := o.ProjectMetadata(super.NewContext(), p)
	require.Len(t, values, 1)
	require.Equal(t, "{b:{d:{min:0.7,max:0.9}},a:{min:1,max:3}}", sup.FormatValue(values[0]))
}

func TestObjectProjectMetadataForUnion(t *testing.T) {
	sctx := super.NewContext()
	supValues := []string{
		"{a:1::(int64|string),b?:2,c:3::(int64|null)}",
		`{a:"s"::(int64|string),b?:_::int64,c:null::(int64|null)}`,
	}
	builder := vector.NewDynamicValueBuilder()
	for _, s := range supValues {
		builder.Write(sup.MustParseValue(sctx, s))
	}
	var b bytes.Buffer
	w := csup.NewSerializer(sio.NopCloser(&b))
	require.NoError(t, w.Push(builder.Build(sctx)))
	require.NoError(t, w.Close())
	csupBytes := b.Bytes()

	o, err := csup.NewObject(bytes.NewReader(csupBytes))
	require.NoError(t, err)
	p := field.NewProjection(field.DottedList("a,b,c"))
	values := o.ProjectMetadata(super.NewContext(), p)
	require.Len(t, values, 1)
	require.Equal(t, `{a:{min:1,max:"s"},b:{min:2,max:2},c:{min:3,max:3}}`, sup.FormatValue(values[0]))
}
