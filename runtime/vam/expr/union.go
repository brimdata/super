package expr

import (
	"slices"

	"github.com/brimdata/super"
	"github.com/brimdata/super/vector"
	"github.com/brimdata/super/vector/vbuild"
)

func AppendMissingToUnion(sctx *super.Context, utyp *super.TypeUnion, vecs []vector.Any) []vector.Any {
	// Add missing vectors.
	for _, typ := range utyp.Types {
		i := slices.IndexFunc(vecs, func(vec vector.Any) bool {
			return vec.Type() == typ
		})
		if i == -1 {
			vecs = append(vecs, vbuild.NewEmpty(sctx, typ))
		}
	}
	return vecs
}
