package dbio

import (
	"github.com/brimdata/super"
	"github.com/brimdata/super/db"
	"github.com/brimdata/super/db/commits"
	"github.com/brimdata/super/db/data"
	"github.com/brimdata/super/db/pools"
	"github.com/brimdata/super/pkg/field"
	"github.com/brimdata/super/runtime/sam/op/meta"
)

var unmarshaler *super.Unmarshaler

func init() {
	unmarshaler = super.NewUnmarshaler()
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
