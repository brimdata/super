package function

import (
	"encoding/base64"
	"encoding/hex"

	"github.com/brimdata/super"
)

// https://github.com/brimdata/super/blob/main/docs/language/functions.md#base64
type Base64 struct {
	sctx *super.Context
}

func (b *Base64) Call(_ super.Allocator, args []super.Value) super.Value {
	val := args[0].Under()
	switch val.Type().ID() {
	case super.IDBytes:
		if val.IsNull() {
			return b.sctx.NewErrorf("base64: illegal null argument")
		}
		return super.NewString(base64.StdEncoding.EncodeToString(val.Bytes()))
	case super.IDString:
		if val.IsNull() {
			return super.NullBytes
		}
		bytes, err := base64.StdEncoding.DecodeString(super.DecodeString(val.Bytes()))
		if err != nil {
			return b.sctx.WrapError("base64: string argument is not base64", val)
		}
		return super.NewBytes(bytes)
	default:
		return b.sctx.WrapError("base64: argument must a bytes or string type", val)
	}
}

// https://github.com/brimdata/super/blob/main/docs/language/functions.md#hex
type Hex struct {
	sctx *super.Context
}

func (h *Hex) Call(_ super.Allocator, args []super.Value) super.Value {
	val := args[0].Under()
	switch val.Type().ID() {
	case super.IDBytes:
		if val.IsNull() {
			return h.sctx.NewErrorf("hex: illegal null argument")
		}
		return super.NewString(hex.EncodeToString(val.Bytes()))
	case super.IDString:
		if val.IsNull() {
			return super.NullBytes
		}
		b, err := hex.DecodeString(super.DecodeString(val.Bytes()))
		if err != nil {
			return h.sctx.WrapError("hex: string argument is not hexidecimal", val)
		}
		return super.NewBytes(b)
	default:
		return h.sctx.WrapError("base64: argument must a bytes or string type", val)
	}
}
