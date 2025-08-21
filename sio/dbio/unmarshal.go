package dbio

import (
	"github.com/brimdata/super/db"
	"github.com/brimdata/super/db/commits"
	"github.com/brimdata/super/db/data"
	"github.com/brimdata/super/db/pools"
	"github.com/brimdata/super/pkg/field"
	"github.com/brimdata/super/runtime/sam/op/meta"
	"github.com/brimdata/super/sup"
)

var unmarshaler *sup.UnmarshalBSUPContext

func init() {
	unmarshaler = sup.NewBSUPUnmarshaler()
	unmarshaler.Bind(
		commits.Add{},
		commits.Commit{},
		commits.Delete{},
		field.Path{},
		meta.Partition{},
		pools.Config{},
		db.BranchMeta{},
		db.BranchTip{},
		data.Object{},
	)
}
