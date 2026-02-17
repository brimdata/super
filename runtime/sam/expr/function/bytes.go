package function

import (
	"encoding/base64"
	"encoding/hex"

	"github.com/brimdata/super"
)

type Base64 struct {
	sctx *super.Context
}

func (b *Base64) Call(args []super.Value) super.Value {
	val := args[0].Under()
	if val.IsNull() {
		return super.Null
	}
	switch val.Type().ID() {
	case super.IDBytes:
		return super.NewString(base64.StdEncoding.EncodeToString(val.Bytes()))
	case super.IDString:
		bytes, err := base64.StdEncoding.DecodeString(super.DecodeString(val.Bytes()))
		if err != nil {
			return b.sctx.WrapError("base64: string argument is not base64", val)
		}
		return super.NewBytes(bytes)
	default:
		return b.sctx.WrapError("base64: argument must a bytes or string type", val)
	}
}

type Hex struct {
	sctx *super.Context
}

func (h *Hex) Call(args []super.Value) super.Value {
	val := args[0].Under()
	if val.IsNull() {
		return super.Null
	}
	switch val.Type().ID() {
	case super.IDBytes:
		return super.NewString(hex.EncodeToString(val.Bytes()))
	case super.IDString:
		b, err := hex.DecodeString(super.DecodeString(val.Bytes()))
		if err != nil {
			return h.sctx.WrapError("hex: string argument is not hexidecimal", val)
		}
		return super.NewBytes(b)
	default:
		return h.sctx.WrapError("base64: argument must a bytes or string type", val)
	}
}
