package queryio

import (
	"github.com/brimdata/super/api"
	"github.com/brimdata/super/tsup"
)

var unmarshaler *tsup.UnmarshalBSUPContext

func init() {
	unmarshaler = tsup.NewBSUPUnmarshaler()
	unmarshaler.Bind(
		api.QueryChannelSet{},
		api.QueryChannelEnd{},
		api.QueryError{},
		api.QueryStats{},
		api.QueryWarning{},
	)
}
