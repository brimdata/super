package queryio

import (
	"github.com/superdb/super"
	"github.com/superdb/super/api"
)

var unmarshaler *super.Unmarshaler

func init() {
	unmarshaler = super.NewUnmarshaler()
	unmarshaler.Bind(
		api.QueryChannelSet{},
		api.QueryChannelEnd{},
		api.QueryError{},
		api.QueryStats{},
		api.QueryWarning{},
	)
}
