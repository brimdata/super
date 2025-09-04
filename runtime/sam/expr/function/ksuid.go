package function

import (
	"github.com/brimdata/super"
	"github.com/segmentio/ksuid"
)

type KSUIDToString struct {
	sctx *super.Context
}

func (k *KSUIDToString) Call(args []super.Value) super.Value {
	if len(args) == 0 {
		return super.NewBytes(ksuid.New().Bytes())
	}
	val := args[0]
	switch val.Type().ID() {
	case super.IDBytes:
		if val.IsNull() {
			return k.sctx.NewErrorf("ksuid: illegal null argument")
		}
		// XXX GC
		id, err := ksuid.FromBytes(val.Bytes())
		if err != nil {
			panic(err)
		}
		return super.NewString(id.String())
	case super.IDString:
		// XXX GC
		id, err := ksuid.Parse(string(val.Bytes()))
		if err != nil {
			return k.sctx.WrapError("ksuid: "+err.Error(), val)
		}
		return super.NewBytes(id.Bytes())
	default:
		return k.sctx.WrapError("ksuid: argument must a bytes or string type", val)
	}
}
