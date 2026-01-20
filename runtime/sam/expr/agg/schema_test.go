package agg

import (
	"testing"

	"github.com/brimdata/super"
	"github.com/brimdata/super/sup"
)

func TestSchemaSamePrimitiveTypeTwice(t *testing.T) {
	s := NewSchema(super.NewContext())
	typ := super.TypeInt64
	s.Mixin(typ)
	s.Mixin(typ)
	if sType := s.Type(); sType != typ {
		t.Fatalf("expected %s, got %s", sup.FormatType(typ), sup.FormatType(sType))
	}
}
