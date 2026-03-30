//go:build js && wasm

package jsonskip

import (
	"bytes"
	"encoding/json"
	"errors"
)

func Skip(b []byte) (int, int) {
	dec := json.NewDecoder(bytes.NewReader(b))
	if err := dec.Decode(new(json.RawMessage)); err != nil {
		var offset int
		var serr *json.SyntaxError
		if errors.As(err, &serr) {
			offset = int(serr.Offset)
		}
		return -1, offset
	}
	return 0, int(dec.InputOffset())
}
