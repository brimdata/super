package parquetio

import "sync"

type poolAllocator struct {
	pool sync.Pool
}

func (a *poolAllocator) Allocate(size int) []byte {
	if b, ok := a.pool.Get().([]byte); ok {
		if size <= cap(b) {
			zeroBytes(b[:size])
			return b[:size]
		}
		a.Free(b)
	}
	// Use append to bump the slice capacity up to the size class selected
	// by the Go allocator.
	return append([]uint8(nil), make([]uint8, size)...)
}

func (a *poolAllocator) Reallocate(size int, b []byte) []byte {
	if size <= cap(b) {
		if i := len(b); i < size {
			zeroBytes(b[i:size])
		}
		return b[:size]
	}
	bb := a.Allocate(size)
	copy(bb, b)
	a.Free(b)
	return bb
}

func (a *poolAllocator) Free(b []byte) {
	a.pool.Put(b)
}

func zeroBytes(b []byte) {
	for i := range b {
		b[i] = 0
	}
}
