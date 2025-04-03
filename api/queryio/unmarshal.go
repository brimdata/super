package queryio

import (
	"github.com/brimdata/super/api"
	"github.com/brimdata/super/sup"
)

var unmarshaler *sup.UnmarshalZNGContext

func init() {
	unmarshaler = sup.NewZNGUnmarshaler()
	unmarshaler.Bind(
		api.QueryChannelSet{},
		api.QueryChannelEnd{},
		api.QueryError{},
		api.QueryStats{},
		api.QueryWarning{},
	)
}
