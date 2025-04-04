package queryio

import (
	"github.com/brimdata/super/api"
	"github.com/brimdata/super/sup"
)

var unmarshaler *sup.UnmarshalBSUPContext

func init() {
	unmarshaler = sup.NewBSUPUnmarshaler()
	unmarshaler.Bind(
		api.QueryChannelSet{},
		api.QueryChannelEnd{},
		api.QueryError{},
		api.QueryStats{},
		api.QueryWarning{},
	)
}
