package zio_test

//  This is really a system test dressed up as a unit test.

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/brimdata/super"
	"github.com/brimdata/super/zio"
	"github.com/brimdata/super/zio/supio"
	"github.com/brimdata/super/zio/zjsonio"
	"github.com/brimdata/super/zio/zngio"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type Output struct {
	bytes.Buffer
}

func (o *Output) Close() error {
	return nil
}

// Send logs to SUP reader -> ZNG writer -> ZNG reader -> SUP writer.
func boomerang(t *testing.T, logs string, compress bool) {
	in := []byte(strings.TrimSpace(logs) + "\n")
	supSrc := supio.NewReader(super.NewContext(), bytes.NewReader(in))
	var rawzng Output
	rawDst := zngio.NewWriterWithOpts(&rawzng, zngio.WriterOpts{
		Compress:    compress,
		FrameThresh: zngio.DefaultFrameThresh,
	})
	require.NoError(t, zio.Copy(rawDst, supSrc))
	require.NoError(t, rawDst.Close())

	var out Output
	rawSrc := zngio.NewReader(super.NewContext(), &rawzng)
	defer rawSrc.Close()
	supDst := supio.NewWriter(&out, supio.WriterOpts{})
	err := zio.Copy(supDst, rawSrc)
	if assert.NoError(t, err) {
		assert.Equal(t, in, out.Bytes())
	}
}

func boomerangZJSON(t *testing.T, logs string) {
	supSrc := supio.NewReader(super.NewContext(), strings.NewReader(logs))
	var zjsonOutput Output
	zjsonDst := zjsonio.NewWriter(&zjsonOutput)
	err := zio.Copy(zjsonDst, supSrc)
	require.NoError(t, err)

	var out Output
	zjsonSrc := zjsonio.NewReader(super.NewContext(), &zjsonOutput)
	supDst := supio.NewWriter(&out, supio.WriterOpts{})
	err = zio.Copy(supDst, zjsonSrc)
	if assert.NoError(t, err) {
		assert.Equal(t, strings.TrimSpace(logs), strings.TrimSpace(out.String()))
	}
}

const sup1 = `
{foo:|["\"test\""]|}
{foo:|["\"testtest\""]|}
`

const sup2 = `{foo:{bar:"test"}}`

const sup3 = "{foo:|[null(string)]|}"

const sup4 = `{foo:"-"}`

const sup5 = `{foo:"[",bar:"[-]"}`

// Make sure we handle null fields and empty sets.
const sup6 = "{id:{a:null(string),s:|[]|(|[string]|)}}"

// Make sure we handle empty and null sets.
const sup7 = `{a:"foo",b:|[]|(|[string]|),c:null(|[string]|)}`

// recursive record with null set and empty set
const sup8 = `
{id:{a:null(string),s:|[]|(|[string]|)}}
{id:{a:null(string),s:null(|[string]|)}}
{id:null({a:string,s:|[string]|})}
`

// generate some really big strings
func supBig() string {
	return fmt.Sprintf(`{f0:"%s",f1:"%s",f2:"%s",f3:"%s"}`,
		"aaaa", strings.Repeat("b", 400), strings.Repeat("c", 30000), "dd")
}

func TestRaw(t *testing.T) {
	boomerang(t, sup1, false)
	boomerang(t, sup2, false)
	boomerang(t, sup3, false)
	boomerang(t, sup4, false)
	boomerang(t, sup5, false)
	boomerang(t, sup6, false)
	boomerang(t, sup7, false)
	boomerang(t, sup8, false)
	boomerang(t, supBig(), false)
}

func TestRawCompressed(t *testing.T) {
	boomerang(t, sup1, true)
	boomerang(t, sup2, true)
	boomerang(t, sup3, true)
	boomerang(t, sup4, true)
	boomerang(t, sup5, true)
	boomerang(t, sup6, true)
	boomerang(t, sup7, true)
	boomerang(t, sup8, true)
	boomerang(t, supBig(), true)
}

func TestZjson(t *testing.T) {
	boomerangZJSON(t, sup1)
	boomerangZJSON(t, sup2)
	// XXX this one doesn't work right now but it's sort of ok becaue
	// it's a little odd to have an null string value inside of a set.
	// semantically this would mean the value shouldn't be in the set,
	// but right now this turns into an empty string, which is somewhat reasonable.
	//boomerangZJSON(t, sup3)
	boomerangZJSON(t, sup4)
	boomerangZJSON(t, sup5)
	boomerangZJSON(t, sup6)
	boomerangZJSON(t, sup7)
	// XXX need to fix bug in json reader where it always uses a primitive null
	// even within a container type (like json array)
	//boomerangZJSON(t, sup8)
	boomerangZJSON(t, supBig())
}

func TestNamed(t *testing.T) {
	const simple = `{foo:"bar",orig_h:127.0.0.1(=ipaddr)}`
	const multipleRecords = `
{foo:"bar",orig_h:127.0.0.1(=ipaddr)}
{foo:"bro",resp_h:127.0.0.1(=ipaddr)}
`
	const recordNamed = `
{foo:{host:127.0.0.2}(=myrec)}
{foo:null(myrec={host:ip})}
`
	t.Run("ZNG", func(t *testing.T) {
		t.Run("simple", func(t *testing.T) {
			boomerang(t, simple, true)
		})
		t.Run("named-type-in-different-records", func(t *testing.T) {
			boomerang(t, multipleRecords, true)
		})
		t.Run("named-record-type", func(t *testing.T) {
			boomerang(t, recordNamed, true)
		})
	})
	t.Run("ZJSON", func(t *testing.T) {
		t.Run("simple", func(t *testing.T) {
			boomerangZJSON(t, simple)
		})
		t.Run("named-type-in-different-records", func(t *testing.T) {
			boomerangZJSON(t, multipleRecords)
		})
		t.Run("named-record-type", func(t *testing.T) {
			boomerangZJSON(t, recordNamed)
		})
	})
}
