package aggregate_test

import (
	"testing"

	"github.com/brimdata/super/runtime/sam/op/aggregate"
	"github.com/brimdata/super/ztest"
)

func TestAggregateZtestsSpill(t *testing.T) {
	saved := aggregate.DefaultLimit
	t.Cleanup(func() { aggregate.DefaultLimit = saved })
	aggregate.DefaultLimit = 1
	ztest.Run(t, "../../../ztests/op/aggregate")
}
