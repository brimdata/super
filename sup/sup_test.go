package sup_test

import (
	"encoding/json"
	"testing"

	"github.com/brimdata/super"
	"github.com/brimdata/super/compiler/ast"
	"github.com/brimdata/super/pkg/fs"
	"github.com/brimdata/super/sup"
	"github.com/brimdata/super/zcode"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func parse(path string) (ast.Value, error) {
	file, err := fs.Open(path)
	if err != nil {
		return nil, err
	}
	return sup.NewParser(file).ParseValue()
}

const testFile = "test.sup"

func TestSUPParser(t *testing.T) {
	val, err := parse(testFile)
	require.NoError(t, err)
	s, err := json.MarshalIndent(val, "", "    ")
	require.NoError(t, err)
	assert.NotEqual(t, s, "")
}

func analyze(sctx *super.Context, path string) (sup.Value, error) {
	val, err := parse(path)
	if err != nil {
		return nil, err
	}
	analyzer := sup.NewAnalyzer()
	return analyzer.ConvertValue(sctx, val)
}

func TestSUPAnalyzer(t *testing.T) {
	sctx := super.NewContext()
	val, err := analyze(sctx, testFile)
	require.NoError(t, err)
	assert.NotNil(t, val)
}

func TestSUPBuilder(t *testing.T) {
	sctx := super.NewContext()
	val, err := analyze(sctx, testFile)
	require.NoError(t, err)
	b := zcode.NewBuilder()
	zv, err := sup.Build(b, val)
	require.NoError(t, err)
	rec := super.NewValue(zv.Type().(*super.TypeRecord), zv.Bytes())
	a := rec.Deref("a")
	assert.Equal(t, `["1","2","3"]`, sup.String(a))
}

func TestFormatPrimitiveNull(t *testing.T) {
	assert.Equal(t, "null", sup.FormatPrimitive(super.TypeString, nil))
}

func TestParseValueStringEscapeSequences(t *testing.T) {
	cases := []struct {
		in       string
		expected string
	}{
		{` "\"\\//\b\f\n\r\t" `, "\"\\//\b\f\n\r\t"},
		{` "\u0000\u000A\u000b" `, "\u0000\u000A\u000b"},
	}
	for _, c := range cases {
		val, err := sup.ParseValue(super.NewContext(), c.in)
		assert.NoError(t, err)
		assert.Equal(t, super.NewString(c.expected), val, "in %q", c.in)
	}
}

func TestParseValueErrors(t *testing.T) {
	cases := []struct {
		in            string
		expectedError string
	}{
		{" \"\n\" ", `parse error: string literal: unescaped line break`},
		{` "`, `parse error: string literal: EOF`},
		{` "\`, `parse error: string literal: no end quote`},
		{` "\u`, `parse error: string literal: EOF`},
		{` "\u" `, `parse error: string literal: short \u escape`},
		{` "\u0" `, `parse error: string literal: short \u escape`},
		{` "\u00" `, `parse error: string literal: short \u escape`},
		{` "\u000" `, `parse error: string literal: short \u escape`},
		{` "\u000g" `, `parse error: string literal: invalid hex digits in \u escape`},
		// Go's \UXXXXXXXX is not recognized.
		{` "\U00000000" `, `parse error: string literal: illegal escape (\U)`},
		// Go's \xXX is not recognized.
		{` "\x00" `, `parse error: string literal: illegal escape (\x)`},
		// Go's \a is not recognized.
		{` "\a" `, `parse error: string literal: illegal escape (\a)`},
		// Go's \v is not recognized.
		{` "\v" `, `parse error: string literal: illegal escape (\v)`},
	}
	for _, c := range cases {
		_, err := sup.ParseValue(super.NewContext(), c.in)
		assert.EqualError(t, err, c.expectedError, "in: %q", c.in)
	}
}
