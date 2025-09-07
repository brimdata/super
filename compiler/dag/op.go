package dag

// This module is derived from the GO AST design pattern in
// https://golang.org/pkg/go/ast/
//
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

import (
	"encoding/json"
	"reflect"
	"slices"

	"github.com/brimdata/super/order"
	"github.com/brimdata/super/pkg/field"
	"github.com/segmentio/ksuid"
)

type Op interface {
	opNode()
}

var PassOp = &Pass{Kind: "Pass"}

type Seq []Op

// Ops

type (
	Aggregate struct {
		Kind         string       `json:"kind" unpack:""`
		Limit        int          `json:"limit"`
		Keys         []Assignment `json:"keys"`
		Aggs         []Assignment `json:"aggs"`
		InputSortDir int          `json:"input_sort_dir,omitempty"`
		PartialsIn   bool         `json:"partials_in,omitempty"`
		PartialsOut  bool         `json:"partials_out,omitempty"`
	}
	// A BadOp node is a placeholder for an expression containing semantic
	// errors.
	BadOp struct {
		Kind string `json:"kind" unpack:""`
	}
	Combine struct {
		Kind string `json:"kind" unpack:""`
	}
	Cut struct {
		Kind string       `json:"kind" unpack:""`
		Args []Assignment `json:"args"`
	}
	Distinct struct {
		Kind string `json:"kind" unpack:""`
		Expr Expr   `json:"expr"`
	}
	Drop struct {
		Kind string `json:"kind" unpack:""`
		Args []Expr `json:"args"`
	}
	Explode struct {
		Kind string `json:"kind" unpack:""`
		Args []Expr `json:"args"`
		Type string `json:"type"`
		As   string `json:"as"`
	}
	Filter struct {
		Kind string `json:"kind" unpack:""`
		Expr Expr   `json:"expr"`
	}
	Fork struct {
		Kind  string `json:"kind" unpack:""`
		Paths []Seq  `json:"paths"`
	}
	Fuse struct {
		Kind string `json:"kind" unpack:""`
	}
	HashJoin struct {
		Kind       string `json:"kind" unpack:""`
		Style      string `json:"style"`
		LeftAlias  string `json:"left_alias"`
		RightAlias string `json:"right_alias"`
		LeftKey    Expr   `json:"left_key"`
		RightKey   Expr   `json:"right_key"`
	}
	Head struct {
		Kind  string `json:"kind" unpack:""`
		Count int    `json:"count"`
	}
	Join struct {
		Kind       string `json:"kind" unpack:""`
		Style      string `json:"style"`
		LeftAlias  string `json:"left_alias"`
		RightAlias string `json:"right_alias"`
		Cond       Expr   `json:"cond"`
	}
	Load struct {
		Kind    string      `json:"kind" unpack:""`
		Pool    ksuid.KSUID `json:"pool"`
		Branch  string      `json:"branch"`
		Author  string      `json:"author"`
		Message string      `json:"message"`
		Meta    string      `json:"meta"`
	}
	Merge struct {
		Kind  string     `json:"kind" unpack:""`
		Exprs []SortExpr `json:"exprs"`
	}
	Mirror struct {
		Kind   string `json:"kind" unpack:""`
		Main   Seq    `json:"main"`
		Mirror Seq    `json:"mirror"`
	}
	Output struct {
		Kind string `json:"kind" unpack:""`
		Name string `json:"name"`
	}
	Pass struct {
		Kind string `json:"kind" unpack:""`
	}
	Put struct {
		Kind string       `json:"kind" unpack:""`
		Args []Assignment `json:"args"`
	}
	Rename struct {
		Kind string       `json:"kind" unpack:""`
		Args []Assignment `json:"args"`
	}
	Scatter struct {
		Kind  string `json:"kind" unpack:""`
		Paths []Seq  `json:"paths"`
	}
	Scope struct {
		Kind  string  `json:"kind" unpack:""`
		Funcs []*Func `json:"funcs"`
		Body  Seq     `json:"seq"`
	}
	Shape struct {
		Kind string `json:"kind" unpack:""`
	}
	Skip struct {
		Kind  string `json:"kind" unpack:""`
		Count int    `json:"count"`
	}
	Sort struct {
		Kind    string     `json:"kind" unpack:""`
		Exprs   []SortExpr `json:"exprs"`
		Reverse bool       `json:"reverse"` // Always false if len(Args)>0.
	}
	Switch struct {
		Kind  string `json:"kind" unpack:""`
		Expr  Expr   `json:"expr"`
		Cases []Case `json:"cases"`
	}
	Tail struct {
		Kind  string `json:"kind" unpack:""`
		Count int    `json:"count"`
	}
	Top struct {
		Kind    string     `json:"kind" unpack:""`
		Limit   int        `json:"limit"`
		Exprs   []SortExpr `json:"exprs"`
		Reverse bool       `json:"reverse"` // Always false if len(Exprs)>0.
	}
	Uniq struct {
		Kind  string `json:"kind" unpack:""`
		Cflag bool   `json:"cflag"`
	}
	Unnest struct {
		Kind string `json:"kind" unpack:""`
		Expr Expr   `json:"exprs"`
		Body Seq    `json:"body"`
	}
	Values struct {
		Kind  string `json:"kind" unpack:""`
		Exprs []Expr `json:"exprs"`
	}
	// Vectorize executes its body using the vector engine.
	Vectorize struct {
		Kind string `json:"kind" unpack:""`
		Body Seq    `json:"body"`
	}
)

// Input structure

type (
	Lister struct {
		Kind      string      `json:"kind" unpack:""`
		Pool      ksuid.KSUID `json:"pool"`
		Commit    ksuid.KSUID `json:"commit"`
		KeyPruner Expr        `json:"key_pruner"`
	}
	Slicer struct {
		Kind string `json:"kind" unpack:""`
	}
	SeqScan struct {
		Kind      string       `json:"kind" unpack:""`
		Pool      ksuid.KSUID  `json:"pool"`
		Commit    ksuid.KSUID  `json:"commit"`
		Fields    []field.Path `json:"fields"`
		Filter    Expr         `json:"filter"`
		KeyPruner Expr         `json:"key_pruner"`
	}
	Deleter struct {
		Kind      string      `json:"kind" unpack:""`
		Pool      ksuid.KSUID `json:"pool"`
		Where     Expr        `json:"where"`
		KeyPruner Expr        `json:"key_pruner"`
	}

	// Sources

	// DefaultScan scans an input stream provided by the runtime.
	DefaultScan struct {
		Kind     string         `json:"kind" unpack:""`
		Filter   Expr           `json:"filter"`
		SortKeys order.SortKeys `json:"sort_keys"`
	}
	FileScan struct {
		Kind     string   `json:"kind"  unpack:""`
		Path     string   `json:"path"`
		Format   string   `json:"format"`
		Pushdown Pushdown `json:"pushdown"`
	}
	Pushdown struct {
		Projection []field.Path `json:"projection"`
		DataFilter *ScanFilter  `json:"data_filter"`
		MetaFilter *ScanFilter  `json:"meta_filter"`
		Unordered  bool         `json:"unordered"`
	}
	ScanFilter struct {
		Projection []field.Path `json:"projection"`
		Expr       Expr         `json:"expr"`
	}
	HTTPScan struct {
		Kind    string              `json:"kind" unpack:""`
		URL     string              `json:"url"`
		Format  string              `json:"format"`
		Method  string              `json:"method"`
		Headers map[string][]string `json:"headers"`
		Body    string              `json:"body"`
	}
	PoolScan struct {
		Kind   string      `json:"kind" unpack:""`
		ID     ksuid.KSUID `json:"id"`
		Commit ksuid.KSUID `json:"commit"`
	}
	RobotScan struct {
		Kind   string `json:"kind" unpack:""`
		Expr   Expr   `json:"expr"`
		Format string `json:"format"`
		Filter Expr   `json:"filter"`
	}
	DeleteScan struct {
		Kind   string      `json:"kind" unpack:""`
		ID     ksuid.KSUID `json:"id"`
		Commit ksuid.KSUID `json:"commit"`
	}
	PoolMetaScan struct {
		Kind string      `json:"kind" unpack:""`
		ID   ksuid.KSUID `json:"id"`
		Meta string      `json:"meta"`
	}
	CommitMetaScan struct {
		Kind      string      `json:"kind" unpack:""`
		Pool      ksuid.KSUID `json:"pool"`
		Commit    ksuid.KSUID `json:"commit"`
		Meta      string      `json:"meta"`
		Tap       bool        `json:"tap"`
		KeyPruner Expr        `json:"key_pruner"`
	}
	DBMetaScan struct {
		Kind string `json:"kind" unpack:""`
		Meta string `json:"meta"`
	}
	NullScan struct {
		Kind string `json:"kind" unpack:""`
	}
)

var DBMetas = map[string]struct{}{
	"branches": {},
	"pools":    {},
}

var PoolMetas = map[string]struct{}{
	"branches": {},
}

var CommitMetas = map[string]struct{}{
	"log":        {},
	"objects":    {},
	"partitions": {},
	"rawlog":     {},
	"vectors":    {},
}

func (*DefaultScan) opNode()    {}
func (*FileScan) opNode()       {}
func (*HTTPScan) opNode()       {}
func (*PoolScan) opNode()       {}
func (*RobotScan) opNode()      {}
func (*DeleteScan) opNode()     {}
func (*DBMetaScan) opNode()     {}
func (*PoolMetaScan) opNode()   {}
func (*CommitMetaScan) opNode() {}
func (*NullScan) opNode()       {}

func (*Lister) opNode()  {}
func (*Slicer) opNode()  {}
func (*SeqScan) opNode() {}
func (*Deleter) opNode() {}

// Various Op fields

type (
	Assignment struct {
		Kind string `json:"kind" unpack:""`
		LHS  Expr   `json:"lhs"`
		RHS  Expr   `json:"rhs"`
	}
	Case struct {
		Expr Expr `json:"expr"`
		Path Seq  `json:"seq"`
	}
)

func (*Aggregate) opNode() {}
func (*BadOp) opNode()     {}
func (*Fork) opNode()      {}
func (*Scatter) opNode()   {}
func (*Switch) opNode()    {}
func (*Sort) opNode()      {}
func (*Cut) opNode()       {}
func (*Distinct) opNode()  {}
func (*Drop) opNode()      {}
func (*Head) opNode()      {}
func (*Tail) opNode()      {}
func (*Skip) opNode()      {}
func (*Pass) opNode()      {}
func (*Filter) opNode()    {}
func (*Uniq) opNode()      {}
func (*Top) opNode()       {}
func (*Put) opNode()       {}
func (*Rename) opNode()    {}
func (*Fuse) opNode()      {}
func (*HashJoin) opNode()  {}
func (*Join) opNode()      {}
func (*Shape) opNode()     {}
func (*Explode) opNode()   {}
func (*Unnest) opNode()    {}
func (*Values) opNode()    {}
func (*Vectorize) opNode() {}
func (*Merge) opNode()     {}
func (*Mirror) opNode()    {}
func (*Combine) opNode()   {}
func (*Scope) opNode()     {}
func (*Load) opNode()      {}
func (*Output) opNode()    {}

// NewFilter returns a filter node for e.
func NewFilter(e Expr) *Filter {
	return &Filter{
		Kind: "Filter",
		Expr: e,
	}
}

func (seq *Seq) Prepend(front Op) {
	*seq = append([]Op{front}, *seq...)
}

func (seq *Seq) Append(op Op) {
	*seq = append(*seq, op)
}

func (seq *Seq) Delete(from, to int) {
	*seq = slices.Delete(*seq, from, to)
}

func FanIn(seq Seq) int {
	if len(seq) == 0 {
		return 0
	}
	switch op := seq[0].(type) {
	case *Fork:
		n := 0
		for _, seq := range op.Paths {
			n += FanIn(seq)
		}
		return n
	case *Scatter:
		n := 0
		for _, seq := range op.Paths {
			n += FanIn(seq)
		}
		return n
	case *Scope:
		return FanIn(op.Body)
	case *Join:
		return 2
	}
	return 1
}

func (t *This) String() string {
	return field.Path(t.Path).String()
}

func CopySeq(seq Seq) Seq {
	var copies Seq
	for _, o := range seq {
		copies = append(copies, CopyOp(o))
	}
	return copies
}

func CopyOp(o Op) Op {
	if o == nil {
		panic("CopyOp nil")
	}
	b, err := json.Marshal(o)
	if err != nil {
		panic(err)
	}
	copy, err := UnmarshalOp(b)
	if err != nil {
		panic(err)
	}
	return copy
}

func WalkT[T any](v reflect.Value, post func(T) T) {
	switch v.Kind() {
	case reflect.Array, reflect.Slice:
		for i := range v.Len() {
			WalkT(v.Index(i), post)
		}
	case reflect.Interface, reflect.Pointer:
		WalkT(v.Elem(), post)
	case reflect.Struct:
		for i := range v.NumField() {
			WalkT(v.Field(i), post)
		}
	}
	if v.CanSet() {
		if t, ok := v.Interface().(T); ok {
			v.Set(reflect.ValueOf(post(t)))
		}
	}
}
