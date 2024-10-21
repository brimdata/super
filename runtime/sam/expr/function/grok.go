package function

import (
	"strings"

	"github.com/brimdata/super"
	"github.com/brimdata/super/pkg/grok"
	"github.com/brimdata/super/zcode"
)

type Grok struct {
	zctx    *super.Context
	builder zcode.Builder
	hosts   map[string]*host
	// fields is used as a scratch space to avoid allocating a new slice.
	fields []super.Field
}

func newGrok(zctx *super.Context) *Grok {
	return &Grok{
		zctx:  zctx,
		hosts: make(map[string]*host),
	}
}

func (g *Grok) Call(_ super.Allocator, args []super.Value) super.Value {
	patternArg, inputArg, defArg := args[0], args[1], super.NullString
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
	h, err := g.getHost(defArg.AsString())
	if err != nil {
		return g.error(err.Error(), defArg)
	}
	p, err := h.getPattern(patternArg.AsString())
	if err != nil {
		return g.error(err.Error(), patternArg)
	}
	keys, vals := p.ParseKeyValues(inputArg.AsString())
	if vals == nil {
		return g.error("value does not match pattern", inputArg)
	}
	g.fields = g.fields[:0]
	for _, key := range keys {
		g.fields = append(g.fields, super.NewField(key, super.TypeString))
	}
	typ := g.zctx.MustLookupTypeRecord(g.fields)
	g.builder.Reset()
	for _, s := range vals {
		g.builder.Append([]byte(s))
	}
	return super.NewValue(typ, g.builder.Bytes())
}

func (g *Grok) error(msg string, val super.Value) super.Value {
	return g.zctx.WrapError("grok(): "+msg, val)
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
