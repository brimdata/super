//go:build !(js && wasm)

package jsonskip

import "github.com/bytedance/sonic/decoder"

func Skip(b []byte) (int, int) {
	return decoder.Skip(b)
}
