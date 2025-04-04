package function

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/brimdata/super"
	"github.com/brimdata/super/sup"
	"github.com/brimdata/super/zio/supio"
)

// https://github.com/brimdata/super/blob/main/docs/language/functions.md#parse_uri
type ParseURI struct {
	zctx      *super.Context
	marshaler *sup.MarshalBSUPContext
}

func NewParseURI(zctx *super.Context) *ParseURI {
	return &ParseURI{zctx, sup.NewBSUPMarshalerWithContext(zctx)}
}

func (p *ParseURI) Call(_ super.Allocator, args []super.Value) super.Value {
	type uri struct {
		Scheme   *string    `zed:"scheme"`
		Opaque   *string    `zed:"opaque"`
		User     *string    `zed:"user"`
		Password *string    `zed:"password"`
		Host     *string    `zed:"host"`
		Port     *uint16    `zed:"port"`
		Path     *string    `zed:"path"`
		Query    url.Values `zed:"query"`
		Fragment *string    `zed:"fragment"`
	}
	in := args[0]
	if !in.IsString() {
		return p.zctx.WrapError("parse_uri: string arg required", in)
	}
	if in.IsNull() {
		out, err := p.marshaler.Marshal((*uri)(nil))
		if err != nil {
			panic(err)
		}
		return out
	}
	s := super.DecodeString(in.Bytes())
	u, err := url.Parse(s)
	if err != nil {
		return p.zctx.WrapError("parse_uri: "+err.Error(), in)
	}
	var v uri
	if u.Scheme != "" {
		v.Scheme = &u.Scheme
	}
	if u.Opaque != "" {
		v.Opaque = &u.Opaque
	}
	if s := u.User.Username(); s != "" {
		v.User = &s
	}
	if s, ok := u.User.Password(); ok {
		v.Password = &s
	}
	if s := u.Hostname(); s != "" {
		v.Host = &s
	}
	if portString := u.Port(); portString != "" {
		u64, err := strconv.ParseUint(portString, 10, 16)
		if err != nil {
			return p.zctx.WrapError(fmt.Sprintf("parse_uri: invalid port %q", portString), in)
		}
		u16 := uint16(u64)
		v.Port = &u16
	}
	if u.Path != "" {
		v.Path = &u.Path
	}
	if q := u.Query(); len(q) > 0 {
		v.Query = q
	}
	if u.Fragment != "" {
		v.Fragment = &u.Fragment
	}
	out, err := p.marshaler.Marshal(v)
	if err != nil {
		panic(err)
	}
	return out
}

// https://github.com/brimdata/super/blob/main/docs/language/functions.md#parse_sup
type ParseSUP struct {
	zctx *super.Context
	sr   *strings.Reader
	zr   *supio.Reader
}

func newParseSUP(zctx *super.Context) *ParseSUP {
	var sr strings.Reader
	return &ParseSUP{zctx, &sr, supio.NewReader(zctx, &sr)}
}

func (p *ParseSUP) Call(_ super.Allocator, args []super.Value) super.Value {
	in := args[0].Under()
	if !in.IsString() {
		return p.zctx.WrapError("parse_sup: string arg required", args[0])
	}
	if in.IsNull() {
		return super.Null
	}
	p.sr.Reset(super.DecodeString(in.Bytes()))
	val, err := p.zr.Read()
	if err != nil {
		return p.zctx.WrapError("parse_sup: "+err.Error(), args[0])
	}
	if val == nil {
		return super.Null
	}
	return *val
}
