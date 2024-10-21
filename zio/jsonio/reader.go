package jsonio

import (
	"bufio"
	"errors"
	"fmt"
	"io"

	"github.com/brimdata/super"
	"github.com/brimdata/super/pkg/byteconv"
	"github.com/brimdata/super/pkg/jsonlexer"
	"golang.org/x/text/unicode/norm"
)

type Reader struct {
	builder builder
	lexer   *jsonlexer.Lexer
	buf     []byte
}

func NewReader(zctx *super.Context, r io.Reader) *Reader {
	return &Reader{
		builder: builder{zctx: zctx},
		// 64 KB gave the best performance when this was written.
		lexer: jsonlexer.New(bufio.NewReaderSize(r, 64*1024)),
		// Ensure handleToken never passes a nil buf to
		// builder.pushPrimitiveItem.
		buf: make([]byte, 0, 64),
	}
}

func (r *Reader) Read() (*super.Value, error) {
	t := r.lexer.Token()
	if t == jsonlexer.TokenErr {
		err := r.lexer.Err()
		if err == io.EOF {
			return nil, nil
		}
		return nil, r.error(t, "")
	}
	r.builder.reset()
	if err := r.handleToken("", t); err != nil {
		return nil, err
	}
	return r.builder.value(), nil
}

func (r *Reader) handleToken(fieldName string, t jsonlexer.Token) error {
	r.buf = r.buf[:0]
	switch t {
	case jsonlexer.TokenString:
		b, ok := unquoteBytes(r.lexer.Buf())
		if !ok {
			return fmt.Errorf("invalid JSON string %q", r.lexer.Buf())
		}
		r.buf = norm.NFC.Append(r.buf, b...)
		r.builder.pushPrimitiveItem(fieldName, super.TypeString, r.buf)
	case jsonlexer.TokenNumber:
		if i, err := byteconv.ParseInt64(r.lexer.Buf()); err == nil {
			r.buf = super.AppendInt(r.buf, i)
			r.builder.pushPrimitiveItem(fieldName, super.TypeInt64, r.buf)
		} else if f, err := byteconv.ParseFloat64(r.lexer.Buf()); err == nil {
			r.buf = super.AppendFloat64(r.buf, f)
			r.builder.pushPrimitiveItem(fieldName, super.TypeFloat64, r.buf)
		} else {
			return err
		}
	case jsonlexer.TokenBeginObject:
		r.builder.beginContainer(fieldName)
		if err := r.readRecord(); err != nil {
			return err
		}
		r.builder.endRecord()
	case jsonlexer.TokenBeginArray:
		r.builder.beginContainer(fieldName)
		if err := r.readArray(); err != nil {
			return err
		}
		r.builder.endArray()
	case jsonlexer.TokenNull:
		r.builder.pushPrimitiveItem(fieldName, super.TypeNull, nil)
	case jsonlexer.TokenFalse, jsonlexer.TokenTrue:
		r.buf = super.AppendBool(r.buf, t == jsonlexer.TokenTrue)
		r.builder.pushPrimitiveItem(fieldName, super.TypeBool, r.buf)
	default:
		return r.error(t, "looking for beginning of value")
	}
	return nil
}

func (r *Reader) readArray() error {
	switch t := r.lexer.Token(); t {
	case jsonlexer.TokenEndArray:
		return nil
	default:
		if err := r.handleToken("", t); err != nil {
			return err
		}
	}
	for {
		switch t := r.lexer.Token(); t {
		case jsonlexer.TokenEndArray:
			return nil
		case jsonlexer.TokenValueSeparator:
			if err := r.handleToken("", r.lexer.Token()); err != nil {
				return err
			}
		default:
			return r.error(t, "after array value")
		}
	}
}

func (r *Reader) readRecord() error {
	switch t := r.lexer.Token(); t {
	case jsonlexer.TokenEndObject:
		return nil
	default:
		if err := r.readNameValuePair(t); err != nil {
			return err
		}
	}
	for {
		switch t := r.lexer.Token(); t {
		case jsonlexer.TokenEndObject:
			return nil
		case jsonlexer.TokenValueSeparator:
			if err := r.readNameValuePair(r.lexer.Token()); err != nil {
				return err
			}
		default:
			return r.error(t, "after object key")
		}
	}
}

func (r *Reader) readNameValuePair(t jsonlexer.Token) error {
	if t != jsonlexer.TokenString {
		return r.error(t, "looking for beginning of object key string")
	}
	fieldName, ok := unquote(r.lexer.Buf())
	if !ok {
		return fmt.Errorf("invalid string %q", r.lexer.Buf())
	}
	if t := r.lexer.Token(); t != jsonlexer.TokenNameSeparator {
		return r.error(t, "after object key")
	}
	return r.handleToken(fieldName, r.lexer.Token())
}

func (r *Reader) error(t jsonlexer.Token, msg string) error {
	if t == jsonlexer.TokenErr {
		err := r.lexer.Err()
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			return errors.New("unexpected end of JSON input")
		}
		return err
	}
	return fmt.Errorf("invalid character %q %s", r.lexer.Buf()[0], msg)
}
