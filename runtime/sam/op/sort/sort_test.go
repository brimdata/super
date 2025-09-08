package sort_test

import (
	"fmt"
	"math/rand"
	"sort"
	"strings"
	"testing"

	opsort "github.com/brimdata/super/runtime/sam/op/sort"
	"github.com/brimdata/super/ztest"
)

// Data sets for tests:
const unsortedInts = `
{foo:100}
{foo:2}
{foo:9100}
`

const ascendingInts = `
{foo:2}
{foo:100}
{foo:9100}
`

const descendingInts = `
{foo:9100}
{foo:100}
{foo:2}
`

const unsortedStrings = `
{foo:"zzz"}
{foo:"hello"}
{foo:"abc"}
{foo:"abcd"}
 `
const sortedStrings = `
{foo:"abc"}
{foo:"abcd"}
{foo:"hello"}
{foo:"zzz"}
`

// A point that can be included with unsortedInts
const nullInt = `
{foo:null::int64}
`

// Some records that don't include the field "foo".  These are combined
// with sets that include foo to test mixed records.
const nofoo = `
{notfoo:1}
{notfoo:2}
{notfoo:3}
`

// Records for testing sorting on multiple fields.
const multiIn = `
{foo:5,bar:10}
{foo:10,bar:10}
{foo:10,bar:5}
{foo:5,bar:5}
`

const foobarOut = `
{foo:5,bar:5}
{foo:5,bar:10}
{foo:10,bar:5}
{foo:10,bar:10}
`

const barfooOut = `
{foo:5,bar:5}
{foo:10,bar:5}
{foo:5,bar:10}
{foo:10,bar:10}
`

// Test cases for sort without a field list, in which case sort chooses
// the field to sort on.
// First case: prefer an int-valued field (n in this case)
const chooseIn1 = `
{s:"a",n:300,ts:2019-11-24T15:41:36Z}
{s:"b",n:100,ts:2019-11-24T15:41:35Z}
{s:"c",n:200,ts:2019-11-24T15:41:34Z}
`

const chooseOut1 = `
{s:"b",n:100,ts:2019-11-24T15:41:35Z}
{s:"c",n:200,ts:2019-11-24T15:41:34Z}
{s:"a",n:300,ts:2019-11-24T15:41:36Z}
`

// Second case: prefer a non-time-valued field.
const chooseIn2 = `
{s:"c",ts:2019-11-24T15:41:34Z}
{s:"a",ts:2019-11-24T15:41:36Z}
{s:"b",ts:2019-11-24T15:41:35Z}
`

const chooseOut2 = `
{s:"a",ts:2019-11-24T15:41:36Z}
{s:"b",ts:2019-11-24T15:41:35Z}
{s:"c",ts:2019-11-24T15:41:34Z}
`

// This case: no numeric fields, just take the very first one.
const chooseIn3 = `
{s:"a",s2:"c"}
{s:"c",s2:"a"}
{s:"b",s2:"b"}
`
const chooseOut3 = `
{s:"a",s2:"c"}
{s:"b",s2:"b"}
{s:"c",s2:"a"}
`

func trim(s string) string {
	return strings.TrimSpace(s) + "\n"
}

func cat(ss ...string) string {
	var out string
	for _, s := range ss {
		out += trim(s)
	}
	return out
}

func runTest(t *testing.T, cmd, input, output string) {
	(&ztest.ZTest{
		SPQ:    cmd,
		Input:  &input,
		Output: trim(output),
	}).Run(t, "", "")
}

func TestSort(t *testing.T) {
	// Test simple sorting of integers.
	runTest(t, "sort foo", unsortedInts, ascendingInts)

	// Test sorting ints in reverse.
	runTest(t, "sort -r foo", unsortedInts, descendingInts)

	// Test sorting strings.
	runTest(t, "sort foo", unsortedStrings, sortedStrings)

	// Test that null values are sorted to the end
	runTest(t, "sort foo", unsortedInts+trim(nullInt), ascendingInts+trim(nullInt))

	// Test sorting records that don't all have the requested field.
	missingFields := cat(nofoo, unsortedStrings)
	missingSorted := cat(sortedStrings, nofoo)
	runTest(t, "sort foo", missingFields, missingSorted)

	// Test sorting records with different types.
	mixedTypesIn := cat(unsortedStrings, unsortedInts)
	mixedTypesOut := cat(ascendingInts, sortedStrings)
	runTest(t, "sort foo", mixedTypesIn, mixedTypesOut)

	// Test sorting on multiple fields.
	runTest(t, "sort foo, bar", multiIn, foobarOut)
	runTest(t, "sort bar, foo", multiIn, barfooOut)

	// Test that choosing a field when none is provided works.
	runTest(t, "sort", chooseIn1, chooseOut1)
	runTest(t, "sort", chooseIn2, chooseOut2)
	runTest(t, "sort", chooseIn3, chooseOut3)
}

func TestSortExternal(t *testing.T) {
	saved := opsort.MemMaxBytes
	opsort.MemMaxBytes = 1024
	defer func() {
		opsort.MemMaxBytes = saved
	}()

	makeSUP := func(ss []string) string {
		var b strings.Builder
		for _, s := range ss {
			b.WriteString(fmt.Sprintf("{s:%q}\n", s))
		}
		return b.String()
	}

	// Create enough strings to exceed 2 * proc.SortMemMaxBytes.
	var n int
	var ss []string
	for n <= 2*opsort.MemMaxBytes {
		s := fmt.Sprintf("%016x", rand.Uint64())
		n += len(s)
		ss = append(ss, s)
	}
	input := makeSUP(ss)
	sort.Strings(ss)
	output := makeSUP(ss)
	runTest(t, "sort s", input, output)
}
