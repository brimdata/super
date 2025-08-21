package scode

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

var appendCases = [][][]byte{
	{},
	{nil},
	{[]byte{}},
	{[]byte{}, nil},
	{[]byte("data")},
	{[]byte("\x00\x01\x02")},
	{[]byte("UTF-8 \b5Ὂg̀9!℃ᾭG€�")},
	{[]byte("data"), nil, []byte("\x1a\x2b\x3c"), []byte("UTF-8 \b5Ὂg̀9!℃ᾭG€�")},
	{[]byte("thisisareallylongstringdoyoulikereallylongstrings?Ithoughtyoumightlikethemsoiaddedthistothetest")},
}

func TestAppend(t *testing.T) {
	for _, c := range appendCases {
		var buf []byte
		for _, val := range c {
			buf = Append(buf, val)
		}
		it := Iter(buf)
		for _, expected := range c {
			assert.False(t, it.Done())
			assert.Exactly(t, expected, []byte(it.Next()))
		}
		assert.True(t, it.Done())
	}
}
