package zio

import (
	"bytes"
	"strings"
	"testing"

	"github.com/brimdata/zed/zson"
)

func TestPeeker(t *testing.T) {
	const input = `
{key:"key1",value:"value1"}
{key:"key2",value:"value2"}
{key:"key3",value:"value3"}
{key:"key4",value:"value4"}
{key:"key5",value:"value5"}
{key:"key6",value:"value6"}
`
	stream := zson.NewReader(strings.NewReader(input), zson.NewContext())
	peeker := NewPeeker(stream)
	rec1, err := peeker.Peek()
	if err != nil {
		t.Error(err)
	}
	rec2, err := peeker.Peek()
	if err != nil {
		t.Error(err)
	}
	if !bytes.Equal(rec1.Bytes, rec2.Bytes) {
		t.Error("rec1 != rec2")
	}
	rec3, err := peeker.Read()
	if err != nil {
		t.Error(err)
	}
	if !bytes.Equal(rec1.Bytes, rec3.Bytes) {
		t.Error("rec1 != rec3")
	}
	rec4, err := peeker.Peek()
	if err != nil {
		t.Error(err)
	}
	if bytes.Equal(rec3.Bytes, rec4.Bytes) {
		t.Error("rec3 == rec4")
	}
	rec5, err := peeker.Read()
	if err != nil {
		t.Error(err)
	}
	if !bytes.Equal(rec4.Bytes, rec5.Bytes) {
		t.Error("rec4 != rec5")
	}
}
