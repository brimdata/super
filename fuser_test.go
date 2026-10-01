package super_test

import (
	"testing"

	"github.com/superdb/super"
	"github.com/superdb/super/sup"
)

func TestFuserSamePrimitiveTypeTwice(t *testing.T) {
	s := super.NewFuser(super.NewContext(), false)
	typ := super.TypeInt64
	s.Fuse(typ)
	s.Fuse(typ)
	if sType := s.Type(); sType != typ {
		t.Fatalf("expected %s, got %s", sup.FormatType(typ), sup.FormatType(sType))
	}
}
