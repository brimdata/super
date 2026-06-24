package super_test

import (
	"fmt"
	"testing"

	"github.com/brimdata/super"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestArenaCopy(t *testing.T) {
	t.Run("native value is returned unchanged", func(t *testing.T) {
		var a super.Arena
		c := a.Copy(super.NewInt64(42))
		assert.Equal(t, super.TypeInt64, c.Type())
		assert.Equal(t, int64(42), super.DecodeInt(c.Bytes()))
	})

	t.Run("byte-backed copy is independent of its source", func(t *testing.T) {
		var a super.Arena
		buf := []byte("hello")
		c := a.Copy(super.NewValue(super.TypeString, buf))
		// Overwriting the original backing must not change the arena copy.
		copy(buf, "world")
		assert.Equal(t, "hello", string(c.Bytes()))
	})

	t.Run("null preserves nil bytes", func(t *testing.T) {
		var a super.Arena
		c := a.Copy(super.NewValue(super.TypeString, nil))
		assert.Nil(t, []byte(c.Bytes()))
	})

	t.Run("values spanning many chunks all survive", func(t *testing.T) {
		var a super.Arena
		const n = 20000 // ~30 bytes each => well over the 64 KiB chunk size
		copies := make([]super.Value, n)
		want := make([]string, n)
		for i := range copies {
			s := fmt.Sprintf("value-%020d", i)
			want[i] = s
			copies[i] = a.Copy(super.NewValue(super.TypeString, []byte(s)))
		}
		// Every earlier value must survive the chunk allocations triggered by
		// later ones.
		for i, c := range copies {
			require.Equal(t, want[i], string(c.Bytes()))
		}
	})

	t.Run("value larger than a chunk is copied correctly", func(t *testing.T) {
		var a super.Arena
		big := make([]byte, 1<<17) // 128 KiB > chunk size
		for i := range big {
			big[i] = byte(i)
		}
		c := a.Copy(super.NewValue(super.TypeString, big))
		assert.Equal(t, big, []byte(c.Bytes()))
	})
}
