package op

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/brimdata/super"
	"github.com/brimdata/super/runtime"
	"github.com/brimdata/super/runtime/exec"
	"github.com/brimdata/super/runtime/vam/expr"
	"github.com/brimdata/super/sbuf"
	"github.com/brimdata/super/scode"
	"github.com/brimdata/super/sio"
	"github.com/brimdata/super/sio/jsonio"
	"github.com/brimdata/super/vector"
	"github.com/brimdata/super/vector/vio"
)

type Robot struct {
	parent   vio.Puller
	rctx     *runtime.Context
	env      *exec.Environment
	expr     expr.Evaluator
	pushdown sbuf.Pushdown
	format   string
	// HTTP options.  method is constant per query; headers and body are
	// evaluated at runtime, once per input value (the same way the URL expr is).
	method      string
	headersExpr expr.Evaluator
	bodyExpr    expr.Evaluator

	vec        vector.Any
	headersVec vector.Any
	bodyVec    vector.Any
	off        uint32
	src        vio.Puller
}

func NewRobot(rctx *runtime.Context, env *exec.Environment, parent vio.Puller, e expr.Evaluator, format, method string, headers, body expr.Evaluator, p sbuf.Pushdown) *Robot {
	return &Robot{
		parent:      parent,
		rctx:        rctx,
		env:         env,
		expr:        e,
		pushdown:    p,
		format:      format,
		method:      method,
		headersExpr: headers,
		bodyExpr:    body,
	}
}

func (o *Robot) Pull(done bool) (vector.Any, error) {
	if done {
		o.reset()
		src := o.src
		o.src = nil
		var err error
		if src != nil {
			_, err = src.Pull(true)
		}
		if _, pullErr := o.parent.Pull(true); err == nil {
			err = pullErr
		}
		return nil, err
	}
	return o.pullNext()
}

func (o *Robot) reset() {
	o.off = 0
	o.vec = nil
	o.headersVec = nil
	o.bodyVec = nil
}

func (o *Robot) pullNext() (vector.Any, error) {
	for {
		puller := o.src
		if puller == nil {
			var err error
			puller, err = o.getPuller()
			if puller == nil || err != nil {
				return nil, err
			}
		}
		b, err := puller.Pull(false)
		if b != nil {
			return b, err
		}
		o.src = nil
		if err != nil {
			return nil, err
		}
		_, err = puller.Pull(true)
		if err != nil {
			return nil, err
		}
	}
}

func (o *Robot) getPuller() (vio.Puller, error) {
	src, err := o.nextPuller()
	o.src = src
	return src, err
}

func (o *Robot) nextPuller() (vio.Puller, error) {
	vec := o.vec
	if vec != nil && o.off >= vec.Len() {
		o.reset()
		vec = nil
	}
	if vec == nil {
		var err error
		if vec, err = o.nextVec(); err != nil {
			return nil, err
		}
		if vec == nil {
			return nil, nil
		}
	}
	off := o.off
	o.off++
	var b scode.Builder
	val := vector.ValueAt(&b, vec, off)
	if !val.IsString() {
		return o.errOnVal(vector.Pick(vec, []uint32{off})), nil
	}
	return o.open(off, val.AsString())
}

func (o *Robot) errOnVal(vec vector.Any) vio.Puller {
	out := vector.NewWrappedError(o.rctx.Sctx, "from ecountered non-string input", vec)
	return vio.NewPuller(out)
}

func (o *Robot) nextVec() (vector.Any, error) {
	in, err := o.parent.Pull(false)
	if err != nil {
		return nil, err
	}
	if in == nil {
		o.reset()
		return nil, nil
	}
	// Evaluate the URL expr and (if present) the HTTP header/body exprs against
	// the same input vector so they stay aligned by offset.
	o.vec = o.expr.Eval(in)
	if o.headersExpr != nil {
		o.headersVec = o.headersExpr.Eval(in)
	}
	if o.bodyExpr != nil {
		o.bodyVec = o.bodyExpr.Eval(in)
	}
	o.off = 0
	return o.vec, nil
}

func (o *Robot) open(off uint32, path string) (vio.Puller, error) {
	// This check for attached database will be removed when we add support for pools here.
	if o.env.IsAttached() {
		return nil, fmt.Errorf("%s: cannot open in a database environment", path)
	}
	// When HTTP options are present, drive the request through the HTTP client
	// (method/headers/body), evaluating headers/body for this input value.
	if o.method != "" || o.headersVec != nil || o.bodyVec != nil {
		return o.openHTTP(off, path)
	}
	return o.env.VectorOpen(o.rctx.Context, o.rctx.Sctx, path, o.format, o.pushdown, 1)
}

func (o *Robot) openHTTP(off uint32, url string) (vio.Puller, error) {
	method := o.method
	if method == "" {
		method = http.MethodGet
	}
	var headers http.Header
	if o.headersVec != nil {
		var b scode.Builder
		h, err := unmarshalHeaders(vector.ValueAt(&b, o.headersVec, off))
		if err != nil {
			return nil, err
		}
		headers = h
	}
	var body io.Reader
	if o.bodyVec != nil {
		var b scode.Builder
		bval := vector.ValueAt(&b, o.bodyVec, off)
		// A string body is sent verbatim (e.g. an already-formatted urlencoded
		// payload); any other value is serialized to JSON so a record can be
		// posted directly with `body this`.
		if super.TypeUnder(bval.Type()) == super.TypeString {
			body = strings.NewReader(bval.AsString())
		} else {
			s, err := valueToJSON(bval)
			if err != nil {
				return nil, err
			}
			body = strings.NewReader(s)
		}
	}
	p, err := o.env.OpenHTTP(o.rctx.Context, o.rctx.Sctx, url, o.format, method, headers, body, nil)
	if err != nil {
		return nil, err
	}
	return sbuf.NewDematerializer(o.rctx.Sctx, p), nil
}

func valueToJSON(val super.Value) (string, error) {
	var buf bytes.Buffer
	w := jsonio.NewWriter(sio.NopCloser(&buf), jsonio.WriterOpts{})
	if err := w.Write(val); err != nil {
		return "", err
	}
	if err := w.Close(); err != nil {
		return "", err
	}
	return strings.TrimRight(buf.String(), "\n"), nil
}

func unmarshalHeaders(val super.Value) (http.Header, error) {
	if !super.IsRecordType(val.Type()) {
		return nil, errors.New("headers value must be a record")
	}
	headers := http.Header{}
	for i, f := range val.Fields() {
		fieldVal := val.DerefByColumn(i)
		if fieldVal.IsMissing() {
			continue
		}
		headerStrings, err := decodeStrings(fieldVal)
		if err != nil {
			return nil, err
		}
		headers[f.Name] = append(headers[f.Name], headerStrings...)
	}
	return headers, nil
}

func decodeStrings(val *super.Value) ([]string, error) {
	typ := super.TypeUnder(val.Type())
	if inner := super.InnerType(typ); inner != nil {
		if inner.ID() != super.IDString {
			return nil, errors.New("array elements of header field must be strings")
		}
		var out []string
		for it := val.ContainerIter(); !it.Done(); {
			out = append(out, super.DecodeString(it.Next()))
		}
		return out, nil
	}
	if typ != super.TypeString {
		return nil, errors.New("header field value must be a string or an array or set of strings")
	}
	return []string{val.AsString()}, nil
}
