package dag

// This module is derived from the GO AST design pattern in
// https://golang.org/pkg/go/ast/
//
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

import (
	"encoding/json"
	"slices"

	"github.com/brimdata/super/order"
	"github.com/brimdata/super/pkg/field"
	"github.com/segmentio/ksuid"
)

type Op interface {
	OpNode()
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
	Head struct {
		Kind  string `json:"kind" unpack:""`
		Count int    `json:"count"`
	}
	Join struct {
		Kind     string          `json:"kind" unpack:""`
		Style    string          `json:"style"`
		LeftKey  Expr            `json:"left_key"`
		LeftDir  order.Direction `json:"left_dir"`
		RightKey Expr            `json:"right_key"`
		RightDir order.Direction `json:"right_dir"`
		Args     []Assignment    `json:"args"`
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
	Over struct {
		Kind  string `json:"kind" unpack:""`
		Defs  []Def  `json:"defs"`
		Exprs []Expr `json:"exprs"`
		Vars  []Def  `json:"vars"`
		Body  Seq    `json:"body"`
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
		Kind   string  `json:"kind" unpack:""`
		Consts []Def   `json:"consts"`
		Funcs  []*Func `json:"funcs"`
		Body   Seq     `json:"seq"`
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
	// Vectorize executes its body using the vector engine.
	Vectorize struct {
		Kind string `json:"kind" unpack:""`
		Body Seq    `json:"body"`
	}
	Yield struct {
		Kind  string `json:"kind" unpack:""`
		Exprs []Expr `json:"exprs"`
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
	LakeMetaScan struct {
		Kind string `json:"kind" unpack:""`
		Meta string `json:"meta"`
	}
	NullScan struct {
		Kind string `json:"kind" unpack:""`
	}
)

var LakeMetas = map[string]struct{}{
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

func (*DefaultScan) OpNode()    {}
func (*FileScan) OpNode()       {}
func (*HTTPScan) OpNode()       {}
func (*PoolScan) OpNode()       {}
func (*RobotScan) OpNode()      {}
func (*DeleteScan) OpNode()     {}
func (*LakeMetaScan) OpNode()   {}
func (*PoolMetaScan) OpNode()   {}
func (*CommitMetaScan) OpNode() {}
func (*NullScan) OpNode()       {}

func (*Lister) OpNode()  {}
func (*Slicer) OpNode()  {}
func (*SeqScan) OpNode() {}
func (*Deleter) OpNode() {}

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
	Def struct {
		Name string `json:"name"`
		Expr Expr   `json:"expr"`
	}
)

func (*Aggregate) OpNode() {}
func (*BadOp) OpNode()     {}
func (*Fork) OpNode()      {}
func (*Scatter) OpNode()   {}
func (*Switch) OpNode()    {}
func (*Sort) OpNode()      {}
func (*Cut) OpNode()       {}
func (*Distinct) OpNode()  {}
func (*Drop) OpNode()      {}
func (*Head) OpNode()      {}
func (*Tail) OpNode()      {}
func (*Skip) OpNode()      {}
func (*Pass) OpNode()      {}
func (*Filter) OpNode()    {}
func (*Uniq) OpNode()      {}
func (*Top) OpNode()       {}
func (*Put) OpNode()       {}
func (*Rename) OpNode()    {}
func (*Fuse) OpNode()      {}
func (*Join) OpNode()      {}
func (*Shape) OpNode()     {}
func (*Explode) OpNode()   {}
func (*Over) OpNode()      {}
func (*Vectorize) OpNode() {}
func (*Yield) OpNode()     {}
func (*Merge) OpNode()     {}
func (*Mirror) OpNode()    {}
func (*Combine) OpNode()   {}
func (*Scope) OpNode()     {}
func (*Load) OpNode()      {}
func (*Output) OpNode()    {}

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
