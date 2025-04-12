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
	// format a message based on the contents in the scanner buffer
	// (could also track column and line number)
	return fmt.Errorf("parse error: %s", msg)
}
