package ast

type Type interface {
	Node
	typeNode()
}

type (
	TypePrimitive struct {
		Kind string `json:"kind" unpack:""`
		Name string `json:"name"`
		Loc  `json:"loc"`
	}
	TypeRecord struct {
		Kind   string      `json:"kind" unpack:""`
		Fields []TypeField `json:"fields"`
		Loc    `json:"loc"`
	}
	TypeField struct {
		Name string `json:"name"`
		Type Type   `json:"type"`
		Loc  `json:"loc"`
	}
	TypeArray struct {
		Kind string `json:"kind" unpack:""`
		Type Type   `json:"type"`
		Loc  `json:"loc"`
	}
	TypeSet struct {
		Kind string `json:"kind" unpack:""`
		Type Type   `json:"type"`
		Loc  `json:"loc"`
	}
	TypeUnion struct {
		Kind  string `json:"kind" unpack:""`
		Types []Type `json:"types"`
		Loc   `json:"loc"`
	}
	TypeEnum struct {
		Kind    string  `json:"kind" unpack:""`
		Symbols []*Text `json:"symbols"`
		Loc     `json:"loc"`
	}
	TypeMap struct {
		Kind    string `json:"kind" unpack:""`
		KeyType Type   `json:"key_type"`
		ValType Type   `json:"val_type"`
		Loc     `json:"loc"`
	}
	TypeNull struct {
		Kind       string `json:"kind" unpack:""`
		KeywordPos int    `json:"pos"`
		Loc        `json:"loc"`
	}
	TypeError struct {
		Kind string `json:"kind" unpack:""`
		Type Type   `json:"type"`
		Loc  `json:"loc"`
	}
	TypeName struct {
		Kind string `json:"kind" unpack:""`
		Name string `json:"name"`
		Loc  `json:"loc"`
	}
	TypeDef struct {
		Kind string `json:"kind" unpack:""`
		Name string `json:"name"`
		Type Type   `json:"type"`
		Loc  `json:"loc"`
	}
)

func (*TypePrimitive) typeNode() {}
func (*TypeRecord) typeNode()    {}
func (*TypeArray) typeNode()     {}
func (*TypeSet) typeNode()       {}
func (*TypeUnion) typeNode()     {}
func (*TypeEnum) typeNode()      {}
func (*TypeMap) typeNode()       {}
func (*TypeNull) typeNode()      {}
func (*TypeError) typeNode()     {}
func (*TypeName) typeNode()      {}
func (*TypeDef) typeNode()       {}
