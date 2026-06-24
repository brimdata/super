package super

import "github.com/brimdata/super/scode"

// arenaChunkSize is the size of each backing chunk an Arena allocates. Values
// larger than this get their own chunk.
const arenaChunkSize = 1 << 16 // 64 KiB

// Arena copies the byte payloads of non-native Values into large contiguous
// chunks. Retaining many values then costs O(total bytes / arenaChunkSize)
// allocations instead of one bytes.Clone per value. A chunk is never
// reallocated once it holds a value, so values copied earlier stay valid as
// later ones are appended; when a chunk runs out of room a fresh one is
// allocated and the old one is kept alive by the values that reference it.
//
// The zero Arena is ready to use. An Arena is not safe for concurrent use.
type Arena struct {
	chunk scode.Bytes
}

// Copy returns a copy of v whose bytes are owned by the arena, equivalent to
// v.Copy() but without a per-value allocation. Native values carry their
// payload inline and are returned unchanged. A value with no bytes (including
// a null) is returned with its bytes unchanged, preserving the nil-vs-empty
// distinction that bytes.Clone would.
func (a *Arena) Copy(v Value) Value {
	if _, ok := v.native(); ok {
		return v
	}
	b := v.bytes()
	n := len(b)
	if n == 0 {
		return NewValue(v.Type(), b)
	}
	if cap(a.chunk)-len(a.chunk) < n {
		size := arenaChunkSize
		if n > size {
			size = n
		}
		a.chunk = make(scode.Bytes, 0, size)
	}
	off := len(a.chunk)
	a.chunk = append(a.chunk, b...)
	// Three-index slice so appends to the returned value's bytes can never
	// scribble into the arena's spare capacity.
	return NewValue(v.Type(), a.chunk[off:off+n:off+n])
}
