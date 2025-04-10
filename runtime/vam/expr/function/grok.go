package function

import (
	"strings"

	"github.com/brimdata/super"
	"github.com/brimdata/super/pkg/grok"
	"github.com/brimdata/super/vector"
	"github.com/brimdata/super/zcode"
)

type Grok struct {
	sctx    *super.Context
	builder zcode.Builder
	hosts   map[string]*host
	// fields is used as a scratch space to avoid allocating a new slice.
	fields []super.Field
}

func newGrok(sctx *super.Context) *Grok {
	return &Grok{
		sctx:  sctx,
		hosts: make(map[string]*host),
	}
}

func (g *Grok) Call(args ...vector.Any) vector.Any {
	patternArg, inputArg := args[0], args[1]
	defArg := vector.Any(vector.NewConst(super.NullString, args[0].Len(), nil))
	if len(args) == 3 {
		defArg = args[2]
	}
	switch {
	case super.TypeUnder(defArg.Type()) != super.TypeString:
		return g.error("definitions argument must be a string", defArg)
	case super.TypeUnder(patternArg.Type()) != super.TypeString:
		return g.error("pattern argument must be a string", patternArg)
	case super.TypeUnder(inputArg.Type()) != super.TypeString:
		return g.error("input argument must be a string", inputArg)
	}
	builder := vector.NewDynamicBuilder()
	var defErrs, patErrs []string
	var defErrsIdx, patErrsIdx, inErrsIdx []uint32
	for i := range patternArg.Len() {
		def, _ := vector.StringValue(defArg, i)
		h, err := g.getHost(def)
		if err != nil {
			defErrs = append(defErrs, err.Error())
			defErrsIdx = append(defErrsIdx, i)
			continue
		}
		pat, isnull := vector.StringValue(patternArg, i)
		if isnull {
			builder.Write(super.NewValue(g.sctx.MustLookupTypeRecord(nil), nil))
			continue
		}
		p, err := h.getPattern(pat)
		if err != nil {
			patErrs = append(patErrs, err.Error())
			patErrsIdx = append(patErrsIdx, i)
			continue
		}
		in, isnull := vector.StringValue(inputArg, i)
		if isnull {
			builder.Write(super.NewValue(g.sctx.MustLookupTypeRecord(nil), nil))
			continue
		}
		keys, vals, match := p.ParseKeyValues(in)
		if !match {
			inErrsIdx = append(inErrsIdx, i)
			continue
		}
		g.fields = g.fields[:0]
		for _, key := range keys {
			g.fields = append(g.fields, super.NewField(key, super.TypeString))
		}
		typ := g.sctx.MustLookupTypeRecord(g.fields)
		g.builder.Reset()
		if len(vals) == 0 {
			// If we have a match but no key/vals return empty record.
			g.builder.Append(nil)
		} else {
			for _, s := range vals {
				g.builder.Append([]byte(s))
			}
		}
		builder.Write(super.NewValue(typ, g.builder.Bytes()))
	}
	combiner := vector.NewCombiner(builder.Build())
	if len(defErrsIdx) > 0 {
		combiner.Add(defErrsIdx, g.errorVec(defErrs, defErrsIdx, defArg))
	}
	if len(patErrsIdx) > 0 {
		combiner.Add(patErrsIdx, g.errorVec(patErrs, patErrsIdx, patternArg))
	}
	if len(inErrsIdx) > 0 {
		combiner.Add(inErrsIdx, g.error("value does not match pattern", vector.Pick(inputArg, inErrsIdx)))
	}
	return combiner.Result()
}

func (g *Grok) errorVec(msgs []string, index []uint32, vec vector.Any) vector.Any {
	s := vector.NewStringEmpty(0, nil)
	for _, m := range msgs {
		s.Append("grok(): " + m)
	}
	return vector.NewVecWrappedError(g.sctx, s, vector.Pick(vec, index))
}

func (g *Grok) error(msg string, vec vector.Any) vector.Any {
	return vector.NewWrappedError(g.sctx, "grok(): "+msg, vec)
}

func (g *Grok) getHost(defs string) (*host, error) {
	h, ok := g.hosts[defs]
	if !ok {
		h = &host{Host: grok.NewBase(), patterns: make(map[string]*grok.Pattern)}
		if err := h.AddFromReader(strings.NewReader(defs)); err != nil {
			return nil, err
		}
		g.hosts[defs] = h
	}
	return h, nil
}

type host struct {
	grok.Host
	patterns map[string]*grok.Pattern
}

func (h *host) getPattern(patternArg string) (*grok.Pattern, error) {
	p, ok := h.patterns[patternArg]
	if !ok {
		var err error
		p, err = h.Host.Compile(patternArg)
		if err != nil {
			return nil, err
		}
		h.patterns[patternArg] = p
	}
	return p, nil
}
