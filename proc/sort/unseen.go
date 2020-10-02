package sort

import (
	"github.com/brimsec/zq/expr"
	"github.com/brimsec/zq/zng"
)

type unseenFieldTracker struct {
	unseenFields map[expr.Evaluator]struct{}
	seenTypes    map[*zng.TypeRecord]bool
}

func newUnseenFieldTracker(fields []expr.Evaluator) *unseenFieldTracker {
	unseen := make(map[expr.Evaluator]struct{})
	// We start out withe unseen map full of all the fields and take
	// them out for each record type we encounter.
	for _, f := range fields {
		unseen[f] = struct{}{}
	}
	return &unseenFieldTracker{
		unseenFields: unseen,
		seenTypes:    make(map[*zng.TypeRecord]bool),
	}
}

func (u *unseenFieldTracker) update(rec *zng.Record) {
	if len(u.unseenFields) == 0 || u.seenTypes[rec.Type] {
		// Either have seen this type or nothing to unsee anymore.
		return
	}
	u.seenTypes[rec.Type] = true
	for field := range u.unseenFields {
		v, _ := field.Eval(rec)
		if !v.IsNil() {
			delete(u.unseenFields, field)
		}
	}
}

func (u *unseenFieldTracker) unseen() []expr.Evaluator {
	var fields []expr.Evaluator
	for f := range u.unseenFields {
		fields = append(fields, f)
	}
	return fields
}
