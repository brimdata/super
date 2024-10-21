package pools

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/lake/data"
	"github.com/brimdata/super/lake/journal"
	"github.com/brimdata/super/order"
	"github.com/brimdata/super/pkg/field"
	"github.com/brimdata/super/pkg/nano"
	"github.com/brimdata/super/pkg/storage"
	"github.com/brimdata/super/zson"
	"github.com/segmentio/ksuid"
)

type Config struct {
	Ts         nano.Ts        `zed:"ts"`
	Name       string         `zed:"name"`
	ID         ksuid.KSUID    `zed:"id"`
	SortKeys   order.SortKeys `zed:"layout"`
	SeekStride int            `zed:"seek_stride"`
	Threshold  int64          `zed:"threshold"`
}

var _ journal.Entry = (*Config)(nil)

func NewConfig(name string, sortKeys order.SortKeys, thresh int64, seekStride int) *Config {
	if sortKeys.IsNil() {
		sortKeys = order.SortKeys{order.NewSortKey(order.Desc, field.Dotted("ts"))}
	}
	if thresh == 0 {
		thresh = data.DefaultThreshold
	}
	if seekStride == 0 {
		seekStride = data.DefaultSeekStride
	}
	return &Config{
		Ts:         nano.Now(),
		Name:       name,
		ID:         ksuid.New(),
		SortKeys:   sortKeys,
		SeekStride: seekStride,
		Threshold:  thresh,
	}
}

func (p *Config) Key() string {
	return p.Name
}

func (p *Config) Path(root *storage.URI) *storage.URI {
	return root.JoinPath(p.ID.String())
}

// This is a temporary hack to get the change in order.SortKey working with
// previous versions. At some point we'll do a migration so we don't have to do
// this.
type marshalConfig struct {
	Ts         nano.Ts     `zed:"ts"`
	Name       string      `zed:"name"`
	ID         ksuid.KSUID `zed:"id"`
	SortKey    oldSortKey  `zed:"layout"`
	SeekStride int         `zed:"seek_stride"`
	Threshold  int64       `zed:"threshold"`
}

type oldSortKey struct {
	Order order.Which `json:"order" zed:"order"`
	Keys  field.List  `json:"keys" zed:"keys"`
}

var hackedBindings = []zson.Binding{
	{Name: "order.SortKey", Template: oldSortKey{}},
	{Name: "pools.Config", Template: marshalConfig{}},
}

func (p Config) MarshalZNG(ctx *zson.MarshalZNGContext) (super.Type, error) {
	ctx.NamedBindings(hackedBindings)
	m := marshalConfig{
		Ts:         p.Ts,
		Name:       p.Name,
		ID:         p.ID,
		SeekStride: p.SeekStride,
		Threshold:  p.Threshold,
	}
	if !p.SortKeys.IsNil() {
		m.SortKey.Order = p.SortKeys[0].Order
		for _, sortKey := range p.SortKeys {
			m.SortKey.Keys = append(m.SortKey.Keys, sortKey.Key)
		}
	}
	typ, err := ctx.MarshalValue(&m)
	return typ, err
}

func (p *Config) UnmarshalZNG(ctx *zson.UnmarshalZNGContext, val super.Value) error {
	ctx.NamedBindings(hackedBindings)
	var m marshalConfig
	if err := ctx.Unmarshal(val, &m); err != nil {
		return err
	}
	p.Ts = m.Ts
	p.Name = m.Name
	p.ID = m.ID
	p.SeekStride = m.SeekStride
	p.Threshold = m.Threshold
	for _, k := range m.SortKey.Keys {
		p.SortKeys = append(p.SortKeys, order.NewSortKey(m.SortKey.Order, k))
	}
	return nil
}
