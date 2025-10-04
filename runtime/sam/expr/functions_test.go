package expr_test

import (
	"testing"

	"github.com/brimdata/super/runtime/sam/expr/function"
)

func TestBadFunction(t *testing.T) {
	testCompilationError(t, "notafunction()", function.ErrNoSuchFunction)
}

func TestAbs(t *testing.T) {
	const record = "{u:50::uint64}"

	testSuccessful(t, "abs(-5)", record, "5")
	testSuccessful(t, "abs(5)", record, "5")
	testSuccessful(t, "abs(-3.2)", record, "3.2")
	testSuccessful(t, "abs(3.2)", record, "3.2")
	testSuccessful(t, "abs(u)", record, "50::uint64")

	testCompilationError(t, "abs()", function.ErrTooFewArgs)
	testCompilationError(t, "abs(1, 2)", function.ErrTooManyArgs)
	testSuccessful(t, `abs("hello")`, record, `error({message:"abs: not a number",on:"hello"})`)
}

func TestSqrt(t *testing.T) {
	const record = "{f:6.25,i:9::int32}"

	testSuccessful(t, "sqrt(4.0)", record, "2.")
	testSuccessful(t, "sqrt(f)", record, "2.5")
	testSuccessful(t, "sqrt(i)", record, "3.")

	testCompilationError(t, "sqrt()", function.ErrTooFewArgs)
	testCompilationError(t, "sqrt(1, 2)", function.ErrTooManyArgs)
	testSuccessful(t, "sqrt(-1)", record, "NaN")
}

func TestMinMax(t *testing.T) {
	const record = "{i:1::uint64,f:2.}"

	// Simple cases
	testSuccessful(t, "min(1, 2, 3)", record, "1")
	testSuccessful(t, "max(1, 2, 3)", record, "3")
	testSuccessful(t, "min(3, 2, 1)", record, "1")
	testSuccessful(t, "max(3, 2, 1)", record, "3")

	// Mixed types work
	testSuccessful(t, "min(i, 2, 3)", record, "1::uint64")
	testSuccessful(t, "min(2, 3, i)", record, "1")
	testSuccessful(t, "max(i, 2, 3)", record, "3::uint64")
	testSuccessful(t, "max(2, 3, i)", record, "3")
	testSuccessful(t, "min(1, -2.0)", record, "-2")
	testSuccessful(t, "min(-2.0, 1)", record, "-2.")
	testSuccessful(t, "max(-1, 2.0)", record, "2")
	testSuccessful(t, "max(2.0, -1)", record, "2.")

	// Fails on invalid types
	testSuccessful(t, `min("hello", 2)`, record, `error({message:"min: not a number",on:"hello"})`)
	testSuccessful(t, `max("hello", 2)`, record, `error({message:"max: not a number",on:"hello"})`)
	testSuccessful(t, `min(1.2.3.4, 2)`, record, `error({message:"min: not a number",on:1.2.3.4})`)
	testSuccessful(t, `max(1.2.3.4, 2)`, record, `error({message:"max: not a number",on:1.2.3.4})`)
}

func TestCeilFloorRound(t *testing.T) {
	testSuccessful(t, "ceil(1.5)", "", "2.")
	testSuccessful(t, "floor(1.5)", "", "1.")
	testSuccessful(t, "round(1.5)", "", "2.")

	testSuccessful(t, "ceil(5)", "", "5")
	testSuccessful(t, "floor(5)", "", "5")
	testSuccessful(t, "round(5)", "", "5")

	testCompilationError(t, "ceil()", function.ErrTooFewArgs)
	testCompilationError(t, "ceil(1, 2)", function.ErrTooManyArgs)
	testCompilationError(t, "floor()", function.ErrTooFewArgs)
	testCompilationError(t, "floor(1, 2)", function.ErrTooManyArgs)
	testCompilationError(t, "round()", function.ErrTooFewArgs)
	testCompilationError(t, "round(1, 2)", function.ErrTooManyArgs)
}

func TestLogPow(t *testing.T) {
	// Math.log() computes natural logarithm.  Rather than writing
	// out long floating point numbers in the parameters or results,
	// use more complex expressions that evaluate to simpler values.
	testSuccessful(t, "log(32) / log(2)", "", "5.")
	testSuccessful(t, "log(32.0) / log(2.0)", "", "5.")

	testSuccessful(t, "pow(10, 2)", "", "100.")
	testSuccessful(t, "pow(4.0, 1.5)", "", "8.")

	testCompilationError(t, "log()", function.ErrTooFewArgs)
	testCompilationError(t, "log(2, 3)", function.ErrTooManyArgs)
	testSuccessful(t, "log(0)", "", `error({message:"log: illegal argument",on:0})`)
	testSuccessful(t, "log(-1)", "", `error({message:"log: illegal argument",on:-1})`)

	testCompilationError(t, "pow()", function.ErrTooFewArgs)
	testCompilationError(t, "pow(2, 3, r)", function.ErrTooManyArgs)
	testSuccessful(t, "pow(-1, 0.5)", "", "NaN")
}

func TestOtherStrFuncs(t *testing.T) {
	testSuccessful(t, `replace("bann", "n", "na")`, "", `"banana"`)
	testCompilationError(t, `replace("foo", "bar")`, function.ErrTooFewArgs)
	testCompilationError(t, `replace("foo", "bar", "baz", "blort")`, function.ErrTooManyArgs)
	testSuccessful(t, `replace("foo", "o", 5)`, "", `error({message:"replace: string arg required",on:5})`)

	testSuccessful(t, `lower("BOO")`, "", `"boo"`)
	testCompilationError(t, `lower()`, function.ErrTooFewArgs)
	testCompilationError(t, `lower("BOO", "HOO")`, function.ErrTooManyArgs)

	testSuccessful(t, `upper("boo")`, "", `"BOO"`)
	testCompilationError(t, `upper()`, function.ErrTooFewArgs)
	testCompilationError(t, `upper("boo", "hoo")`, function.ErrTooManyArgs)

	testSuccessful(t, `trim("  hi  there   ")`, "", `"hi  there"`)
	testCompilationError(t, `trim()`, function.ErrTooFewArgs)
	testCompilationError(t, `trim("  hi  ", "  there  ")`, function.ErrTooManyArgs)
}

func TestLen(t *testing.T) {
	record := "{s:|[1::int32,2::int32,3::int32]|,a:[4::int32,5::int32,6::int32]}"

	testSuccessful(t, "len(s)", record, "3")
	testSuccessful(t, "len(a)", record, "3")

	testCompilationError(t, "len()", function.ErrTooFewArgs)
	testCompilationError(t, `len("foo", "bar")`, function.ErrTooManyArgs)
	testSuccessful(t, "len(5)", record, `error({message:"len: bad type",on:5})`)

	record = `{s:"🍺",bs:0xf09f8dba}`

	testSuccessful(t, `len("foo")`, record, "3")
	testSuccessful(t, `len(s)`, record, "1")
	testSuccessful(t, `len(bs)`, record, "4")
}

func TestCast(t *testing.T) {
	// Constant type argument
	testSuccessful(t, "cast(1, <uint64>)", "", "1::uint64")
	testSuccessful(t, "cast(1, 2)", "", `error({message:"cast target must be a type or type name",on:2})`)

	// Constant name argument
	testSuccessful(t, `cast(1, "my_int64")`, "", "1::=my_int64")
	testSuccessful(t, `cast(1, "uint64")`, "",
		`error({message:"cannot cast to named type: bad type name \"uint64\": primitive type name",on:1})`)

	// Variable type argument
	testSuccessful(t, "cast(1, type)", "{type:<uint64>}", "1::uint64")
	testSuccessful(t, "cast(1, type)", "{type:2}",
		`error({message:"cast target must be a type or type name",on:2})`)

	// Variable name argument
	testSuccessful(t, "cast(1, name)", `{name:"my_int64"}`, "1::=my_int64")
	testSuccessful(t, "cast(1, name)", `{name:"uint64"}`,
		`error({message:"cannot cast to named type: bad type name \"uint64\": primitive type name",on:1})`)
	testCompilationError(t, "cast()", function.ErrTooFewArgs)
	testCompilationError(t, "cast(1, 2, 3)", function.ErrTooManyArgs)
}
