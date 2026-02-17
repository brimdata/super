package agg

import (
	"testing"

	"github.com/brimdata/super"
	"github.com/brimdata/super/sup"
)

func TestFuserSamePrimitiveTypeTwice(t *testing.T) {
	s := NewFuser(super.NewContext())
	typ := super.TypeInt64
	s.Fuse(typ)
	s.Fuse(typ)
	if sType := s.Type(); sType != typ {
		t.Fatalf("expected %s, got %s", sup.FormatType(typ), sup.FormatType(sType))
	}
}
