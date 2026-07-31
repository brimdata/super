package agg

import (
	"testing"

	"github.com/brimdata/super"
	"github.com/brimdata/super/tsup"
)

func TestFuserSamePrimitiveTypeTwice(t *testing.T) {
	s := NewFuser(super.NewContext(), false)
	typ := super.TypeInt64
	s.Fuse(typ)
	s.Fuse(typ)
	if sType := s.Type(); sType != typ {
		t.Fatalf("expected %s, got %s", tsup.FormatType(typ), tsup.FormatType(sType))
	}
}
