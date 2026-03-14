package op

import (
	"context"

	"github.com/brimdata/super/vector"
	"github.com/brimdata/super/vector/vio"
)

type Fork struct {
	router *router
}

func NewFork(ctx context.Context, parent vio.Puller) *Fork {
	f := &Fork{}
	f.router = newRouter(ctx, f, parent)
	return f
}

func (f *Fork) AddBranch() vio.Puller {
	return f.router.addRoute()
}

func (f *Fork) forward(vec vector.Any) bool {
	for _, r := range f.router.routes {
		if !r.send(vec, nil) {
			return false
		}
	}
	return true
}
