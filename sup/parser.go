package sup

import (
	"fmt"
	"io"
)

type Parser struct {
	lexer *Lexer
}

func NewParser(r io.Reader) *Parser {
	return &Parser{NewLexer(r)}
}

func (p *Parser) errorf(msg string, args ...any) error {
	return p.error(fmt.Sprintf(msg, args...))
}

func (p *Parser) error(msg string) error {
	return fmt.Errorf("line %d: parse error: %s", p.lexer.line, msg)
}
