package scode

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuilder(t *testing.T) {
	empty := []byte{}
	null := []byte(nil)
	v1, v2 := []byte("1"), []byte("22")
	t.Run("AppendContainer", func(t *testing.T) {
		b := NewBuilder()
		b.Append(empty)
		expected := Append(nil, empty)
		require.Exactly(t, expected, b.Bytes())
		b.Append(null)
		expected = Append(expected, null)
		require.Exactly(t, expected, b.Bytes())
		b.Append(v1)
		b.Append(v2)
		expected = Append(expected, v1)
		expected = Append(expected, v2)
		require.Exactly(t, expected, b.Bytes())
	})
	t.Run("AppendPrimitive", func(t *testing.T) {
		b := NewBuilder()
		b.Append(empty)
		expected := Append(nil, empty)
		require.Exactly(t, expected, b.Bytes())
		b.Append(null)
		expected = Append(expected, null)
		require.Exactly(t, expected, b.Bytes())
		b.Append(v1)
		b.Append(v2)
		expected = Append(expected, v1)
		expected = Append(expected, v2)
		require.Exactly(t, expected, b.Bytes())
	})
	t.Run("BeginContainer/empty", func(t *testing.T) {
		b := NewBuilder()
		b.BeginContainer()
		b.EndContainer()
		expected := Append(nil, empty)
		require.Exactly(t, expected, b.Bytes())
	})
	t.Run("BeginContainer/nonempty", func(t *testing.T) {
		b := NewBuilder()
		big := bytes.Repeat([]byte("x"), 256)
		v1, v2, v3, v4 := []byte("1"), []byte("22"), []byte("333"), []byte("4444")
		b.Append(v1)
		b.BeginContainer() // Start of outer container.
		b.Append(v2)
		b.BeginContainer() // Start of inner container.
		b.Append(big)
		b.EndContainer() // End of inner container.
		b.Append(v3)
		b.EndContainer() // End of outer container.
		b.Append(v4)
		innerContainer := Append(nil, big)
		outerContainer := Append(nil, v2)
		outerContainer = Append(outerContainer, innerContainer)
		outerContainer = Append(outerContainer, v3)
		expected := Append(nil, v1)
		expected = Append(expected, outerContainer)
		expected = Append(expected, v4)
		require.Exactly(t, expected, b.Bytes())
	})
	t.Run("BeginContainer/panic", func(t *testing.T) {
		b := NewBuilder()
		b.BeginContainer()
		require.Panics(t, func() { b.Bytes() })
	})
	t.Run("EndContainer", func(t *testing.T) {
		b := NewBuilder()
		require.Panics(t, func() { b.EndContainer() })
	})
	t.Run("TransformContainer/empty", func(t *testing.T) {
		b := NewBuilder()
		b.BeginContainer()
		b.TransformContainer(func(body Bytes) Bytes { return Append(nil, v1) })
		b.EndContainer()
		expected := Append(nil, Append(nil, v1))
		require.Exactly(t, expected, b.Bytes())
	})
	t.Run("TransformContainer/nonempty", func(t *testing.T) {
		b := NewBuilder()
		b.BeginContainer()
		b.Append(v1)
		b.Append(v2)
		b.TransformContainer(func(body Bytes) Bytes { return Append(nil, v2) })
		b.EndContainer()
		expected := Append(nil, Append(nil, v2))
		require.Exactly(t, expected, b.Bytes())
	})
	t.Run("TransformContainer/panic", func(t *testing.T) {
		b := NewBuilder()
		require.Panics(t, func() {
			b.TransformContainer(func(body Bytes) Bytes { return nil })
		})
	})
	t.Run("Reset", func(t *testing.T) {
		b := NewBuilder()
		b.Append([]byte("1"))
		b.Reset()
		require.Nil(t, b.Bytes())
	})
}
