// Package ast declares the types used to represent syntax trees for SuperSQL
// queries.
package ast

// This module is derived from the GO AST design pattern in
// https://golang.org/pkg/go/ast/
//
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

type Node interface {
	Pos() int // Position of first character belonging to the node.
	End() int // Position of first character immediately after the node.
}

type Loc struct {
	First int `json:"first"`
	Last  int `json:"last"`
}

func NewLoc(pos, end int) Loc {
	return Loc{pos, end}
}

func (l Loc) Pos() int { return l.First }
func (l Loc) End() int { return l.Last }
