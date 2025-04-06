package field

import (
	"fmt"
)

// A Projection is a slice of string or Forks
type Projection []any //XXX clean this up later
type Fork []Projection

func NewProjection(paths []Path) Projection {
	var out Projection
	for _, path := range paths {
		out = insertPath(out, path)
	}
	return out
}

// XXX this is N*N in path lengths... fix?
func insertPath(existing Projection, addition Path) Projection {
	if len(addition) == 0 {
		return existing
	}
	if len(existing) == 0 {
		return convertFieldPath(addition)
	}
	switch elem := existing[0].(type) {
	case string:
		if elem == addition[0] {
			return append(Projection{elem}, insertPath(existing[1:], addition[1:])...)
		}
		return Projection{Fork{existing, convertFieldPath(addition)}}
	case Fork:
		return Projection{addToFork(elem, addition)}
	default:
		panic(fmt.Sprintf("bad type encounted in insertPath: %T", elem))
	}
}

func addToFork(fork Fork, addition Path) Fork {
	// The first element of each path in a fork must be the key distinguishing
	// the different paths (so no embedded Fork as the first element of a fork)
	for k, path := range fork {
		if path[0].(string) == addition[0] {
			fork[k] = insertPath(path, addition)
			return fork
		}
	}
	// No common prefix so add the addition to the fork.
	return append(fork, convertFieldPath(addition))
}

func convertFieldPath(path Path) Projection {
	var out []any
	for _, s := range path {
		out = append(out, s)
	}
	return out
}
