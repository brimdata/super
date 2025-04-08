package function

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/vector"
)

// https://github.com/brimdata/super/blob/main/docs/language/functions.md#fields
type Fields struct {
	sctx     *super.Context
	innerTyp *super.TypeArray
	outerTyp *super.TypeArray
}

func NewFields(sctx *super.Context) *Fields {
	inner := sctx.LookupTypeArray(super.TypeString)
	return &Fields{
		sctx:     sctx,
		innerTyp: inner,
		outerTyp: sctx.LookupTypeArray(inner),
	}
}

func (f *Fields) Call(args ...vector.Any) vector.Any {
	val := vector.Under(args[0])
	switch typ := val.Type().(type) {
	case *super.TypeRecord:
		paths := buildPath(typ, nil)
		s := vector.NewStringEmpty(val.Len(), nil)
		inOffs, outOffs := []uint32{0}, []uint32{0}
		for i := uint32(0); i < val.Len(); i++ {
			inOffs, outOffs = appendPaths(paths, s, inOffs, outOffs)
		}
		inner := vector.NewArray(f.innerTyp, inOffs, s, nil)
		return vector.NewArray(f.outerTyp, outOffs, inner, nil)
	case *super.TypeOfType:
		var errs []uint32
		s := vector.NewStringEmpty(val.Len(), nil)
		inOffs, outOffs := []uint32{0}, []uint32{0}
		for i := uint32(0); i < val.Len(); i++ {
			b, _ := vector.TypeValueValue(val, i)
			rtyp := f.recordType(b)
			if rtyp == nil {
				errs = append(errs, i)
				continue
			}
			inOffs, outOffs = appendPaths(buildPath(rtyp, nil), s, inOffs, outOffs)
		}
		inner := vector.NewArray(f.innerTyp, inOffs, s, nil)
		out := vector.NewArray(f.outerTyp, outOffs, inner, nil)
		if len(errs) > 0 {
			return vector.Combine(out, errs, vector.NewStringError(f.sctx, "missing", uint32(len(errs))))
		}
		return out
	default:
		return vector.NewStringError(f.sctx, "missing", val.Len())
	}
}

func (f *Fields) recordType(b []byte) *super.TypeRecord {
	typ, err := f.sctx.LookupByValue(b)
	if err != nil {
		return nil
	}
	rtyp, _ := typ.(*super.TypeRecord)
	return rtyp
}

func buildPath(typ *super.TypeRecord, prefix []string) [][]string {
	var out [][]string
	for _, f := range typ.Fields {
		if typ, ok := super.TypeUnder(f.Type).(*super.TypeRecord); ok {
			out = append(out, buildPath(typ, append(prefix, f.Name))...)
		} else {
			out = append(out, append(prefix, f.Name))
		}
	}
	return out
}

func appendPaths(paths [][]string, s *vector.String, inner, outer []uint32) ([]uint32, []uint32) {
	for _, path := range paths {
		for _, f := range path {
			s.Append(f)
		}
		inner = append(inner, inner[len(inner)-1]+uint32(len(path)))
	}
	return inner, append(outer, outer[len(outer)-1]+uint32(len(paths)))
}
