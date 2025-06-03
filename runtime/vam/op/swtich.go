package op

import (
	"context"

	"github.com/RoaringBitmap/roaring"
	"github.com/brimdata/super"
	"github.com/brimdata/super/runtime/vam/expr"
	"github.com/brimdata/super/vector"
)

type Switch struct {
	router *router
	cases  []expr.Evaluator
}

func NewSwitch(ctx context.Context, parent vector.Puller) *Switch {
	s := &Switch{}
	s.router = newRouter(ctx, s, parent)
	return s
}

func (s *Switch) AddCase(e expr.Evaluator) vector.Puller {
	s.cases = append(s.cases, e)
	return s.router.addRoute()
}

func (s *Switch) forward(vec vector.Any) bool {
	doneMap := roaring.New()
	for i, c := range s.cases {
		maskVec := c.Eval(vec)
		boolMap, errMap := expr.BoolMask(maskVec)
		boolMap.AndNot(doneMap)
		errMap.AndNot(doneMap)
		doneMap.Or(boolMap)
		if !errMap.IsEmpty() {
			// Clone because iteration results are undefined if the bitmap is modified.
			for it := errMap.Clone().Iterator(); it.HasNext(); {
				i := it.Next()
				if isErrorMissing(maskVec, i) {
					errMap.Remove(i)
				}
			}
		}
		var vec2 vector.Any
		if errMap.IsEmpty() {
			if boolMap.IsEmpty() {
				continue
			}
			vec2 = vector.Pick(vec, boolMap.ToArray())
		} else if boolMap.IsEmpty() {
			vec2 = vector.Pick(maskVec, errMap.ToArray())
		} else {
			valIndex := boolMap.ToArray()
			errIndex := errMap.ToArray()
			tags := make([]uint32, 0, len(valIndex)+len(errIndex))
			for len(valIndex) > 0 && len(errIndex) > 0 {
				if valIndex[0] < errIndex[0] {
					valIndex = valIndex[1:]
					tags = append(tags, 0)
				} else {
					errIndex = errIndex[1:]
					tags = append(tags, 1)
				}
			}
			tags = append(tags, valIndex...)
			tags = append(tags, errIndex...)
			valVec := vector.Pick(vec, valIndex)
			errVec := vector.Pick(maskVec, errIndex)
			vec2 = vector.NewDynamic(tags, []vector.Any{valVec, errVec})
		}
		if !s.router.routes[i].send(vec2, nil) {
			return false
		}
	}
	return true
}

func isErrorMissing(vec vector.Any, i uint32) bool {
	vec = vector.Under(vec)
	if dynVec, ok := vec.(*vector.Dynamic); ok {
		vec = dynVec.Values[dynVec.Tags[i]]
		i = dynVec.ForwardTagMap()[i]
	}
	errVec, ok := vec.(*vector.Error)
	if !ok {
		return false
	}
	if errVec.Vals.Type().ID() != super.IDString {
		return false
	}
	s, null := vector.StringValue(errVec.Vals, i)
	return !null && s == string(super.Missing)
}
