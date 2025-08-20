package csvio

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPreprocess(t *testing.T) {
	const input = `
,,
"","",""
"""","""",""""
field1,"field"2,field"3" my friend
field4,"field"5 with "multiple" quotes "to" escape,field6
""",""",""" has a couple "" embedded quotes and a , comma",""" """
x,"hello,
"" world , " foo,y
field1,field2,"test eof with quotes"`
	const expected = `
,,
"","",""
"""","""",""""
field1,"field2","field3 my friend"
field4,"field5 with multiple quotes to escape",field6
""",""",""" has a couple "" embedded quotes and a , comma",""" """
x,"hello,
"" world ,  foo",y
field1,field2,"test eof with quotes"`

	p := newPreprocess(strings.NewReader(input), ',')
	var buf bytes.Buffer
	_, err := io.Copy(&buf, p)
	require.NoError(t, err)
	assert.Equal(t, expected, buf.String())
}
