package function

import (
	"net/url"
	"strconv"
	"strings"

	"github.com/brimdata/super"
	"github.com/brimdata/super/zio/zsonio"
	"github.com/brimdata/super/zson"
)

// https://github.com/brimdata/super/blob/main/docs/language/functions.md#parse_uri
type ParseURI struct {
	zctx      *super.Context
	marshaler *zson.MarshalZNGContext
}

func (p *ParseURI) Call(_ super.Allocator, args []super.Value) super.Value {
	in := args[0]
	if !in.IsString() || in.IsNull() {
		return p.zctx.WrapError("parse_uri: non-empty string arg required", in)
	}
	s := super.DecodeString(in.Bytes())
	u, err := url.Parse(s)
	if err != nil {
		return p.zctx.WrapError("parse_uri: "+err.Error(), in)
	}
	var v struct {
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
			return p.zctx.WrapError("parse_uri: invalid port: "+portString, in)
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

// https://github.com/brimdata/super/blob/main/docs/language/functions.md#parse_zson
type ParseZSON struct {
	zctx *super.Context
	sr   *strings.Reader
	zr   *zsonio.Reader
}

func newParseZSON(zctx *super.Context) *ParseZSON {
	var sr strings.Reader
	return &ParseZSON{zctx, &sr, zsonio.NewReader(zctx, &sr)}
}

func (p *ParseZSON) Call(_ super.Allocator, args []super.Value) super.Value {
	in := args[0]
	if !in.IsString() {
		return p.zctx.WrapError("parse_zson: string arg required", in)
	}
	if in.IsNull() {
		return super.Null
	}
	p.sr.Reset(super.DecodeString(in.Bytes()))
	val, err := p.zr.Read()
	if err != nil {
		return p.zctx.WrapError("parse_zson: "+err.Error(), in)
	}
	if val == nil {
		return super.Null
	}
	return *val
}
