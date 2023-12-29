package zio

import (
	"github.com/brimdata/zed"
)

type Mapper struct {
	Reader
	mapper *zed.Mapper
}

func NewMapper(zctx *zed.Context, reader Reader) *Mapper {
	return &Mapper{
		Reader: reader,
		mapper: zed.NewMapper(zctx),
	}
}

func (m *Mapper) Read() (*zed.Value, error) {
	rec, err := m.Reader.Read()
	if err != nil {
		return nil, err
	}
	if rec == nil {
		return nil, nil
	}
	id := zed.TypeID(rec.Type())
	sharedType := m.mapper.Lookup(id)
	if sharedType == nil {
		sharedType, err = m.mapper.Enter(id, rec.Type())
		if err != nil {
			return nil, err
		}
	}
	*rec = *zed.NewValue(sharedType, rec.Bytes())
	return rec, nil
}
