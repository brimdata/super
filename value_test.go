package super_test

import (
	"testing"

	"github.com/brimdata/super"
	"github.com/brimdata/super/scode"
	"github.com/stretchr/testify/assert"
)

func TestNewStringNotNull(t *testing.T) {
	assert.NotNil(t, super.NewString("").Bytes())
}

func BenchmarkValueUnder(b *testing.B) {
	b.Run("primitive", func(b *testing.B) {
		val := super.Null
		b.ResetTimer()
		for b.Loop() {
			val.Under()
		}
	})
	b.Run("named", func(b *testing.B) {
		typ, _ := super.NewContext().LookupTypeNamed("name", super.TypeNull)
		val := super.NewValue(typ, nil)
		b.ResetTimer()
		for b.Loop() {
			val.Under()
		}
	})
}

func TestValueValidate(t *testing.T) {
	recType := super.NewTypeRecord(0, []super.Field{
		super.NewField("f", super.NewTypeSet(0, super.TypeString)),
	})
	t.Run("set/error/duplicate-element", func(t *testing.T) {
		var b scode.Builder
		b.BeginContainer()
		b.Append([]byte("dup"))
		b.Append([]byte("dup"))
		// Don't normalize.
		b.EndContainer()
		val := super.NewValue(recType, b.Bytes())
		assert.EqualError(t, val.Validate(), "invalid BSUP: duplicate set element")
	})
	t.Run("set/error/unsorted-elements", func(t *testing.T) {
		var b scode.Builder
		b.BeginContainer()
		b.Append([]byte("a"))
		b.Append([]byte("z"))
		b.Append([]byte("b"))
		// Don't normalize.
		b.EndContainer()
		val := super.NewValue(recType, b.Bytes())
		assert.EqualError(t, val.Validate(), "invalid BSUP: set elements not sorted")
	})
	t.Run("set/primitive-elements", func(t *testing.T) {
		var b scode.Builder
		b.BeginContainer()
		b.Append([]byte("dup"))
		b.Append([]byte("dup"))
		b.Append([]byte("z"))
		b.Append([]byte("a"))
		b.TransformContainer(super.NormalizeSet)
		b.EndContainer()
		val := super.NewValue(recType, b.Bytes())
		assert.NoError(t, val.Validate())
	})
	t.Run("set/complex-elements", func(t *testing.T) {
		var b scode.Builder
		b.BeginContainer()
		for _, s := range []string{"dup", "dup", "z", "a"} {
			b.BeginContainer()
			b.Append([]byte(s))
			b.EndContainer()
		}
		b.TransformContainer(super.NormalizeSet)
		b.EndContainer()
		r := super.NewValue(
			super.NewTypeRecord(0, []super.Field{
				super.NewField("f", super.NewTypeSet(0, super.NewTypeRecord(0, []super.Field{
					super.NewField("g", super.TypeString),
				}))),
			}),
			b.Bytes())
		assert.NoError(t, r.Validate())
	})
}
