package ast

type Value interface {
	valueNode()
}

type ImpliedValue struct {
	Kind string `json:"kind" unpack:""`
	Of   Any    `json:"of"`
}

type DefValue struct {
	Kind     string `json:"kind" unpack:""`
	Of       Any    `json:"of"`
	TypeName string `json:"type_name"`
}

type CastValue struct {
	Kind string `json:"kind" unpack:""`
	Of   Value  `json:"of"`
	Type Type   `json:"type"`
}

func (*ImpliedValue) valueNode() {}
func (*DefValue) valueNode()     {}
func (*CastValue) valueNode()    {}

type Any interface {
	anyNode()
}

// Only Primitive and TypeValue are used in zed parser so only these are given
// ast.Node features.

type (
	Primitive struct {
		Kind    string `json:"kind" unpack:""`
		Type    string `json:"type"`
		Text    string `json:"text"`
		TextPos int    `json:"text_pos"`
	}
	Record struct {
		Kind   string  `json:"kind" unpack:""`
		Fields []Field `json:"fields"`
	}
	Field struct {
		Name  string `json:"name"`
		Value Value  `json:"value"`
	}
	Array struct {
		Kind     string  `json:"kind" unpack:""`
		Elements []Value `json:"elements"`
	}
	Set struct {
		Kind     string  `json:"kind" unpack:""`
		Elements []Value `json:"elements"`
	}
	Enum struct {
		Kind string `json:"kind" unpack:""`
		Name string `json:"name"`
	}
	Map struct {
		Kind    string  `json:"kind" unpack:""`
		Entries []Entry `json:"entries"`
	}
	Entry struct {
		Key   Value `json:"key"`
		Value Value `json:"value"`
	}
	TypeValue struct {
		Kind   string `json:"kind" unpack:""`
		Lbrack int    `json:"lbrack"`
		Value  Type   `json:"value"`
		Rbrack int    `json:"rbrack"`
	}
	Error struct {
		Kind  string `json:"kind" unpack:""`
		Value Value  `json:"value"`
	}
)

func (*Primitive) anyNode() {}
func (*Record) anyNode()    {}
func (*Array) anyNode()     {}
func (*Set) anyNode()       {}
func (*Enum) anyNode()      {}
func (*Map) anyNode()       {}
func (*TypeValue) anyNode() {}
func (*Error) anyNode()     {}

func (*Primitive) ExprAST() {}
func (*TypeValue) ExprAST() {}

func (*Primitive) ExprDAG() {}
func (*TypeValue) ExprDAG() {}

func (x *Primitive) Pos() int { return x.TextPos }
func (x *TypeValue) Pos() int { return x.Lbrack }

func (x *Primitive) End() int {
	// If Primitive type is string we need to adjust the end for quotations
	// marks since they are currently not captured in the Value string. They
	// should be but this will require a change to the ZSON lexer as well as
	// the Zed parser.
	if x.Type == "string" {
		return x.TextPos + len(x.Text) + 2
	}
	return x.TextPos + len(x.Text)
}
func (x *TypeValue) End() int { return x.Rbrack + 1 }
