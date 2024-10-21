package function

import (
	"encoding/base64"
	"encoding/hex"

	"github.com/brimdata/super"
)

// https://github.com/brimdata/super/blob/main/docs/language/functions.md#base64
type Base64 struct {
	zctx *super.Context
}

func (b *Base64) Call(_ super.Allocator, args []super.Value) super.Value {
	val := args[0].Under()
	switch val.Type().ID() {
	case super.IDBytes:
		if val.IsNull() {
			return b.zctx.NewErrorf("base64: illegal null argument")
		}
		return super.NewString(base64.StdEncoding.EncodeToString(val.Bytes()))
	case super.IDString:
		if val.IsNull() {
			return super.NullBytes
		}
		bytes, err := base64.StdEncoding.DecodeString(super.DecodeString(val.Bytes()))
		if err != nil {
			return b.zctx.WrapError("base64: string argument is not base64", val)
		}
		return super.NewBytes(bytes)
	default:
		return b.zctx.WrapError("base64: argument must a bytes or string type", val)
	}
}

// https://github.com/brimdata/super/blob/main/docs/language/functions.md#hex
type Hex struct {
	zctx *super.Context
}

func (h *Hex) Call(_ super.Allocator, args []super.Value) super.Value {
	val := args[0].Under()
	switch val.Type().ID() {
	case super.IDBytes:
		if val.IsNull() {
			return h.zctx.NewErrorf("hex: illegal null argument")
		}
		return super.NewString(hex.EncodeToString(val.Bytes()))
	case super.IDString:
		if val.IsNull() {
			return super.NullBytes
		}
		b, err := hex.DecodeString(super.DecodeString(val.Bytes()))
		if err != nil {
			return h.zctx.WrapError("hex: string argument is not hexidecimal", val)
		}
		return super.NewBytes(b)
	default:
		return h.zctx.WrapError("base64: argument must a bytes or string type", val)
	}
}
