package optimizer

import (
	"testing"

	"github.com/brimdata/super/compiler/dag"
)

// TestFirstThisPathComponentEmptyPath verifies that a bare this reference (an
// empty Path) does not panic and is reported as having no common first
// component. See brimdata/super#6971.
func TestFirstThisPathComponentEmptyPath(t *testing.T) {
	e := &dag.BinaryExpr{
		Kind: "BinaryExpr",
		Op:   "==",
		LHS:  &dag.ThisExpr{Kind: "This", Path: nil},
		RHS:  &dag.ThisExpr{Kind: "This", Path: []string{"id"}},
	}
	_, ok := firstThisPathComponent(e)
	if ok {
		t.Fatalf("expected ok=false for expression containing a bare this reference")
	}
}

// TestFirstThisPathComponentCommonPrefix verifies the normal case where every
// this reference shares a first path component.
func TestFirstThisPathComponentCommonPrefix(t *testing.T) {
	e := &dag.BinaryExpr{
		Kind: "BinaryExpr",
		Op:   "==",
		LHS:  &dag.ThisExpr{Kind: "This", Path: []string{"ducks", "id"}},
		RHS:  &dag.ThisExpr{Kind: "This", Path: []string{"ducks", "name"}},
	}
	prefix, ok := firstThisPathComponent(e)
	if !ok || prefix != "ducks" {
		t.Fatalf("got prefix=%q ok=%v, want prefix=\"ducks\" ok=true", prefix, ok)
	}
}
