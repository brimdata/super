package zson

import (
	"fmt"
	"io"
)

type Parser struct {
	lexer *Lexer
}

func NewParser(r io.Reader) (*Parser, error) {
	l, err := NewLexer(r)
	if err != nil {
		return nil, err
	}
	return &Parser{l}, nil
}

func (p *Parser) errorf(msg string, args ...interface{}) error {
	return p.error(fmt.Sprintf(msg, args...))
}

func (p *Parser) error(msg string) error {
	// format a message based on the contents in the scanner buffer
	// (could also track column and line number)
	return fmt.Errorf("parse error: %s", msg)
}
