package expr_test

import (
	"fmt"
	"net"
	"strconv"
	"testing"

	"github.com/brimsec/zq/expr/function"
	"github.com/brimsec/zq/pkg/nano"
	"github.com/brimsec/zq/zng"
	"github.com/stretchr/testify/require"
)

func zaddr(addr string) zng.Value {
	parsed := net.ParseIP(addr)
	return zng.Value{zng.TypeIP, zng.EncodeIP(parsed)}
}

func TestBadFunction(t *testing.T) {
	testError(t, "notafunction()", nil, function.ErrNoSuchFunction, "calling nonexistent function")
}

func TestAbs(t *testing.T) {
	record, err := parseOneRecord(`
#0:record[u:uint64]
0:[50;]`)
	require.NoError(t, err)

	testSuccessful(t, "Math.abs(-5)", record, zint64(5))
	testSuccessful(t, "Math.abs(5)", record, zint64(5))
	testSuccessful(t, "Math.abs(-3.2)", record, zfloat64(3.2))
	testSuccessful(t, "Math.abs(3.2)", record, zfloat64(3.2))
	testSuccessful(t, "Math.abs(u)", record, zuint64(50))

	testError(t, "Math.abs()", record, function.ErrTooFewArgs, "abs with no args")
	testError(t, "Math.abs(1, 2)", record, function.ErrTooManyArgs, "abs with too many args")
	testError(t, `Math.abs("hello")`, record, function.ErrBadArgument, "abs with non-number")
}

func TestSqrt(t *testing.T) {
	record, err := parseOneRecord(`
#0:record[f:float64,i:int32]
0:[6.25;9;]`)
	require.NoError(t, err)

	testSuccessful(t, "Math.sqrt(4.0)", record, zfloat64(2.0))
	testSuccessful(t, "Math.sqrt(f)", record, zfloat64(2.5))
	testSuccessful(t, "Math.sqrt(i)", record, zfloat64(3.0))

	testError(t, "Math.sqrt()", record, function.ErrTooFewArgs, "sqrt with no args")
	testError(t, "Math.sqrt(1, 2)", record, function.ErrTooManyArgs, "sqrt with too many args")
	testError(t, "Math.sqrt(-1)", record, function.ErrBadArgument, "sqrt of negative")
}

func TestMinMax(t *testing.T) {
	record, err := parseOneRecord(`
#0:record[i:uint64,f:float64]
0:[1;2;]`)
	require.NoError(t, err)

	// Simple cases
	testSuccessful(t, "Math.min(1)", record, zint64(1))
	testSuccessful(t, "Math.max(1)", record, zint64(1))
	testSuccessful(t, "Math.min(1, 2, 3)", record, zint64(1))
	testSuccessful(t, "Math.max(1, 2, 3)", record, zint64(3))
	testSuccessful(t, "Math.min(3, 2, 1)", record, zint64(1))
	testSuccessful(t, "Math.max(3, 2, 1)", record, zint64(3))

	// Fails with no arguments
	testError(t, "Math.min()", record, function.ErrTooFewArgs, "min with no args")
	testError(t, "Math.max()", record, function.ErrTooFewArgs, "max with no args")

	// Mixed types work
	testSuccessful(t, "Math.min(i, 2, 3)", record, zuint64(1))
	testSuccessful(t, "Math.min(2, 3, i)", record, zint64(1))
	testSuccessful(t, "Math.max(i, 2, 3)", record, zuint64(3))
	testSuccessful(t, "Math.max(2, 3, i)", record, zint64(3))
	testSuccessful(t, "Math.min(1, -2.0)", record, zint64(-2))
	testSuccessful(t, "Math.min(-2.0, 1)", record, zfloat64(-2))
	testSuccessful(t, "Math.max(-1, 2.0)", record, zint64(2))
	testSuccessful(t, "Math.max(2.0, -1)", record, zfloat64(2))

	// Fails on invalid types
	testError(t, `Math.min("hello", 2)`, record, function.ErrBadArgument, "min() on string")
	testError(t, `Math.max("hello", 2)`, record, function.ErrBadArgument, "max() on string")
	testError(t, `Math.min(1.2.3.4, 2)`, record, function.ErrBadArgument, "min() on ip")
	testError(t, `Math.max(1.2.3.4, 2)`, record, function.ErrBadArgument, "max() on ip")

}

func TestCeilFloorRound(t *testing.T) {
	testSuccessful(t, "Math.ceil(1.5)", nil, zfloat64(2))
	testSuccessful(t, "Math.floor(1.5)", nil, zfloat64(1))
	testSuccessful(t, "Math.round(1.5)", nil, zfloat64(2))

	testSuccessful(t, "Math.ceil(5)", nil, zint64(5))
	testSuccessful(t, "Math.floor(5)", nil, zint64(5))
	testSuccessful(t, "Math.round(5)", nil, zint64(5))

	testError(t, "Math.ceil()", nil, function.ErrTooFewArgs, "ceil() with no args")
	testError(t, "Math.ceil(1, 2)", nil, function.ErrTooManyArgs, "ceil() with too many args")
	testError(t, "Math.floor()", nil, function.ErrTooFewArgs, "floor() with no args")
	testError(t, "Math.floor(1, 2)", nil, function.ErrTooManyArgs, "floor() with too many args")
	testError(t, "Math.round()", nil, function.ErrTooFewArgs, "round() with no args")
	testError(t, "Math.round(1, 2)", nil, function.ErrTooManyArgs, "round() with too many args")
}

func TestLogPow(t *testing.T) {
	// Math.log() computes natural logarithm.  Rather than writing
	// out long floating point numbers in the parameters or results,
	// use more complex expressions that evaluate to simpler values.
	testSuccessful(t, "Math.log(32) / Math.log(2)", nil, zfloat64(5))
	testSuccessful(t, "Math.log(32.0) / Math.log(2.0)", nil, zfloat64(5))

	testSuccessful(t, "Math.pow(10, 2)", nil, zfloat64(100))
	testSuccessful(t, "Math.pow(4.0, 1.5)", nil, zfloat64(8))

	testError(t, "Math.log()", nil, function.ErrTooFewArgs, "log() with no args")
	testError(t, "Math.log(2, 3)", nil, function.ErrTooManyArgs, "log() with too many args")
	testError(t, "Math.log(0)", nil, function.ErrBadArgument, "log() of 0")
	testError(t, "Math.log(-1)", nil, function.ErrBadArgument, "log() of negative number")

	testError(t, "Math.pow()", nil, function.ErrTooFewArgs, "pow() with no args")
	testError(t, "Math.pow(2, 3, r)", nil, function.ErrTooManyArgs, "pow() with too many args")
	testError(t, "Math.pow(-1, 0.5)", nil, function.ErrBadArgument, "pow() with invalid arguments")
}

func TestMod(t *testing.T) {
	record, err := parseOneRecord(`
#0:record[u:uint64]
0:[5;]`)
	require.NoError(t, err)

	testSuccessful(t, "Math.mod(5, 3)", record, zint64(2))
	testSuccessful(t, "Math.mod(u, 3)", record, zuint64(2))
	testSuccessful(t, "Math.mod(8, 5)", record, zint64(3))
	testSuccessful(t, "Math.mod(8, u)", record, zint64(3))

	testError(t, "Math.mod()", record, function.ErrTooFewArgs, "mod() with no args")
	testError(t, "Math.mod(1, 2, 3)", record, function.ErrTooManyArgs, "mod() with too many args")
	testError(t, "Math.mod(3.2, 2)", record, function.ErrBadArgument, "mod() with float64 arg")
}

func TestStrFormat(t *testing.T) {
	testSuccessful(t, "String.formatFloat(1.2)", nil, zstring("1.2"))
	testError(t, "String.formatFloat()", nil, function.ErrTooFewArgs, "formatFloat() with no args")
	testError(t, "String.formatFloat(1.2, 3.4)", nil, function.ErrTooManyArgs, "formatFloat() with too many args")
	testError(t, "String.formatFloat(1)", nil, function.ErrBadArgument, "formatFloat() with non-float arg")

	testSuccessful(t, "String.formatInt(5)", nil, zstring("5"))
	testError(t, "String.formatInt()", nil, function.ErrTooFewArgs, "formatInt() with no args")
	testError(t, "String.formatInt(3, 4)", nil, function.ErrTooManyArgs, "formatInt() with too many args")
	testError(t, "String.formatInt(1.5)", nil, function.ErrBadArgument, "formatInt() with non-int arg")

	testSuccessful(t, "String.formatIp(1.2.3.4)", nil, zstring("1.2.3.4"))
	testError(t, "String.formatIp()", nil, function.ErrTooFewArgs, "formatIp() with no args")
	testError(t, "String.formatIp(1.2, 3.4)", nil, function.ErrTooManyArgs, "formatIp() with too many args")
	testError(t, "String.formatIp(1)", nil, function.ErrBadArgument, "formatIp() with non-ip arg")
}

func TestStrParse(t *testing.T) {
	testSuccessful(t, `String.parseInt("1")`, nil, zint64(1))
	testSuccessful(t, `String.parseInt("-1")`, nil, zint64(-1))
	testError(t, `String.parseInt()`, nil, function.ErrTooFewArgs, "parseInt() with no args")
	testError(t, `String.parseInt("a", "b")`, nil, function.ErrTooManyArgs, "parseInt() with too many args")
	testError(t, `String.parseInt("abc")`, nil, strconv.ErrSyntax, "parseInt() with non-parseable string")

	testSuccessful(t, `String.parseFloat("5.5")`, nil, zfloat64(5.5))
	testError(t, `String.parseFloat()`, nil, function.ErrTooFewArgs, "parseFloat() with no args")
	testError(t, `String.parseFloat("a", "b")`, nil, function.ErrTooManyArgs, "parseFloat() with too many args")
	testError(t, `String.parseFloat("abc")`, nil, strconv.ErrSyntax, "parseFloat() with non-parseable string")

	testSuccessful(t, `String.parseIp("1.2.3.4")`, nil, zaddr("1.2.3.4"))
	testError(t, `String.parseIp()`, nil, function.ErrTooFewArgs, "parseIp() with no args")
	testError(t, `String.parseIp("a", "b")`, nil, function.ErrTooManyArgs, "parseIp() with too many args")
	testError(t, `String.parseIp("abc")`, nil, function.ErrBadArgument, "parseIp() with non-parseable string")
}

func TestOtherStrFuncs(t *testing.T) {
	testSuccessful(t, `String.replace("bann", "n", "na")`, nil, zstring("banana"))
	testError(t, `String.replace("foo", "bar")`, nil, function.ErrTooFewArgs, "replace() with too few args")
	testError(t, `String.replace("foo", "bar", "baz", "blort")`, nil, function.ErrTooManyArgs, "replace() with too many args")
	testError(t, `String.replace("foo", "o", 5)`, nil, function.ErrBadArgument, "replace() with non-string arg")

	testSuccessful(t, `String.toLower("BOO")`, nil, zstring("boo"))
	testError(t, `String.toLower()`, nil, function.ErrTooFewArgs, "toLower() with no args")
	testError(t, `String.toLower("BOO", "HOO")`, nil, function.ErrTooManyArgs, "toLower() with too many args")

	testSuccessful(t, `String.toUpper("boo")`, nil, zstring("BOO"))
	testError(t, `String.toUpper()`, nil, function.ErrTooFewArgs, "toUpper() with no args")
	testError(t, `String.toUpper("boo", "hoo")`, nil, function.ErrTooManyArgs, "toUpper() with too many args")

	testSuccessful(t, `String.trim("  hi  there   ")`, nil, zstring("hi  there"))
	testError(t, `String.trim()`, nil, function.ErrTooFewArgs, "trim() with no args")
	testError(t, `String.trim("  hi  ", "  there  ")`, nil, function.ErrTooManyArgs, "trim() with too many args")
}

func TestLen(t *testing.T) {
	record, err := parseOneRecord(`
#0:record[s:set[int32],a:array[int32]]
0:[[1;2;3;][4;5;6;]]`)
	require.NoError(t, err)

	testSuccessful(t, "len(s)", record, zint64(3))
	testSuccessful(t, "len(a)", record, zint64(3))

	testError(t, "len()", record, function.ErrTooFewArgs, "len() with no args")
	testError(t, `len("foo", "bar")`, record, function.ErrTooManyArgs, "len() with too many args")
	testError(t, `len("foo")`, record, function.ErrBadArgument, "len() with string arg")
	testError(t, "len(5)", record, function.ErrBadArgument, "len() with non-container arg")

	record, err = parseOneRecord(`
#0:record[s:string,bs:bstring,bs2:bstring]
0:[🍺;\xf0\x9f\x8d\xba;\xba\x8d\x9f\xf0;]`)
	require.NoError(t, err)

	testSuccessful(t, `String.byteLen("foo")`, record, zint64(3))
	testSuccessful(t, `String.byteLen(s)`, record, zint64(4))
	testSuccessful(t, `String.byteLen(bs)`, record, zint64(4))
	testSuccessful(t, `String.byteLen(bs2)`, record, zint64(4))

	testSuccessful(t, `String.runeLen("foo")`, record, zint64(3))
	testSuccessful(t, `String.runeLen(s)`, record, zint64(1))
	testSuccessful(t, `String.runeLen(bs)`, record, zint64(1))
	testSuccessful(t, `String.runeLen(bs2)`, record, zint64(4))
}

func TestTime(t *testing.T) {
	// These represent the same time (Tue, 26 May 2020 15:27:47.967 in GMT)
	iso := "2020-05-26T15:27:47.967Z"
	msec := 1590506867_967
	nsec := msec * 1_000_000
	zval := zng.Value{zng.TypeTime, zng.EncodeTime(nano.Ts(nsec))}

	exp := fmt.Sprintf(`Time.fromISO("%s")`, iso)
	testSuccessful(t, exp, nil, zval)
	exp = fmt.Sprintf("Time.fromMilliseconds(%d)", msec)
	testSuccessful(t, exp, nil, zval)
	exp = fmt.Sprintf("Time.fromMilliseconds(%d.0)", msec)
	testSuccessful(t, exp, nil, zval)
	exp = fmt.Sprintf("Time.fromMicroseconds(%d)", msec*1000)
	testSuccessful(t, exp, nil, zval)
	exp = fmt.Sprintf("Time.fromMicroseconds(%d.0)", msec*1000)
	testSuccessful(t, exp, nil, zval)
	exp = fmt.Sprintf("Time.fromNanoseconds(%d)", nsec)
	testSuccessful(t, exp, nil, zval)
	testSuccessful(t, "Time.trunc(1590506867.967, 1)", nil, zng.Value{zng.TypeTime, zng.EncodeTime(nano.Ts(1590506867 * 1_000_000_000))})

	testError(t, "Time.fromISO()", nil, function.ErrTooFewArgs, "Time.fromISO() with no args")
	testError(t, `Time.fromISO("abc", "def")`, nil, function.ErrTooManyArgs, "Time.fromISO() with too many args")
	testError(t, "Time.fromISO(1234)", nil, function.ErrBadArgument, "Time.fromISO() with wrong argument type")

	testError(t, "Time.fromMilliseconds()", nil, function.ErrTooFewArgs, "Time.fromMilliseconds() with no args")
	testError(t, "Time.fromMilliseconds(123, 456)", nil, function.ErrTooManyArgs, "Time.fromMilliseconds() with too many args")
	testError(t, `Time.fromMilliseconds("1234")`, nil, function.ErrBadArgument, "Time.fromMilliseconds() with wrong argument type")

	testError(t, "Time.fromMicroseconds()", nil, function.ErrTooFewArgs, "Time.fromMicroseconds() with no args")
	testError(t, "Time.fromMicroseconds(123, 456)", nil, function.ErrTooManyArgs, "Time.fromMicroseconds() with too many args")
	testError(t, `Time.fromMicroseconds("1234")`, nil, function.ErrBadArgument, "Time.fromMicroseconds() with wrong argument type")

	testError(t, "Time.fromNanoseconds()", nil, function.ErrTooFewArgs, "Time.fromNanoseconds() with no args")
	testError(t, "Time.fromNanoseconds(123, 456)", nil, function.ErrTooManyArgs, "Time.fromNanoseconds() with too many args")
	testError(t, `Time.fromNanoseconds("1234")`, nil, function.ErrBadArgument, "Time.fromNanoseconds() with wrong argument type")
}
