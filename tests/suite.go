package tests

import (
	"github.com/mccanne/zq/pkg/test"
	"github.com/mccanne/zq/tests/suite/count"
	"github.com/mccanne/zq/tests/suite/cut"
	"github.com/mccanne/zq/tests/suite/diropt"
	"github.com/mccanne/zq/tests/suite/errors"
	"github.com/mccanne/zq/tests/suite/format"
	"github.com/mccanne/zq/tests/suite/input"
	"github.com/mccanne/zq/tests/suite/sort"
	"github.com/mccanne/zq/tests/suite/utf8"
)

var RootDir = "./test-root"

var internals = []test.Internal{
	count.Internal,
	cut.Internal,
	format.Internal,
	format.JsonTypes,
	input.Internal,
	errors.DuplicateFields,
	errors.ErrNotScalar,
	errors.ErrNotScalarZJSON,
	errors.ErrNotContainer,
	errors.ErrNotContainerZJSON,
	errors.ErrMissingField,
	errors.ErrExtraField,
	sort.Internal1,
	sort.Internal2,
	sort.Internal3,
}

var commands = []test.Exec{
	cut.Exec,
	utf8.Exec,
}

var scripts = []test.Shell{
	diropt.Test,
	diropt.Test2,
}
