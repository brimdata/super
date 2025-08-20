package sio

import "github.com/brimdata/super"

// Peeker wraps a Stream while adding a Peek method, which allows inspection
// of the next item to be read without actually reading it.
type Peeker struct {
	Reader
	cache *super.Value
}

func NewPeeker(reader Reader) *Peeker {
	return &Peeker{Reader: reader}
}

func (p *Peeker) Peek() (*super.Value, error) {
	var err error
	if p.cache == nil {
		p.cache, err = p.Reader.Read()
	}
	return p.cache, err
}

func (p *Peeker) Read() (*super.Value, error) {
	v := p.cache
	if v != nil {
		p.cache = nil
		return v, nil
	}
	return p.Reader.Read()
}
