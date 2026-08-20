package function

import (
	"testing"

	"github.com/brimdata/super"
	"github.com/brimdata/super/sup"
	"github.com/stretchr/testify/assert"
)

func TestParseSUPIsolatesParserState(t *testing.T) {
	parse := newParseSUP(super.NewContext())

	got := parse.Call([]super.Value{super.NewString(",")})
	assert.Equal(t, `error({message:"parse_sup: line 1: syntax error",on:","})`, sup.FormatValue(got))

	got = parse.Call([]super.Value{super.NewString("{a:1}")})
	assert.Equal(t, "{a:1}", sup.FormatValue(got))
}
