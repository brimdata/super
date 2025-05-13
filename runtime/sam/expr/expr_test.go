package expr_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/brimdata/super"
	"github.com/brimdata/super/ztest"
)

func testSuccessful(t *testing.T, e, input, expected string) {
	t.Helper()
	if input == "" {
		input = "{}"
	}
	zt := ztest.ZTest{
		Zed:    fmt.Sprintf("yield %s", e),
		Input:  input,
		Output: expected + "\n",
	}
	if err := zt.RunInternal(); err != nil {
		t.Fatal(err)
	}
}

func testError(t *testing.T, e string, expectErr error) {
	t.Helper()
	zt := ztest.ZTest{
		Zed:   fmt.Sprintf("yield %s", e),
		Error: expectErr.Error() + "\n",
	}
	if err := zt.RunInternal(); err != nil {
		t.Fatal(err)
	}
}

func testCompilationError(t *testing.T, e string, expectErr error) {
	t.Helper()
	err := fmt.Errorf(`%s at line 1, column 7:
yield %s
      %s`,
		expectErr, e, strings.Repeat("~", len(e)))
	testError(t, e, err)
}

func TestPrimitives(t *testing.T) {
	const record = `{x:10 (int32),f:2.5,s:"hello"} (=0)`

	// Test simple literals
	testSuccessful(t, "50", record, "50")
	testSuccessful(t, "3.14", record, "3.14")
	testSuccessful(t, `"boo"`, record, `"boo"`)

	// Test good field references
	testSuccessful(t, "x", record, "10(int32)")
	testSuccessful(t, "f", record, "2.5")
	testSuccessful(t, "s", record, `"hello"`)
}

func TestCompareNumbers(t *testing.T) {
	var numericTypes = []string{
		"uint8", "int16", "uint16", "int32", "uint32", "int64", "uint64", "float16", "float32", "float64"}
	var intFields = []string{"u8", "i16", "u16", "i32", "u32", "i64", "u64"}

	for _, typ := range numericTypes {
		// Make a test point with this in a field called x plus
		// one field of each other integer type
		one := "1"
		if strings.HasPrefix(typ, "float") {
			one = "1."
		}
		record := fmt.Sprintf(
			"{x:%s (%s),u8:0 (uint8),i16:0 (int16),u16:0 (uint16),i32:0 (int32),u32:0 (uint32),i64:0,u64:0 (uint64)} (=0)",
			one, typ)
		// Test the 6 comparison operators against a constant
		testSuccessful(t, "x == 1", record, "true")
		testSuccessful(t, "x == 0", record, "false")
		testSuccessful(t, "x != 0", record, "true")
		testSuccessful(t, "x != 1", record, "false")
		testSuccessful(t, "x < 2", record, "true")
		testSuccessful(t, "x < 1", record, "false")
		testSuccessful(t, "x <= 2", record, "true")
		testSuccessful(t, "x <= 1", record, "true")
		testSuccessful(t, "x <= 0", record, "false")
		testSuccessful(t, "x > 0", record, "true")
		testSuccessful(t, "x > 1", record, "false")
		testSuccessful(t, "x >= 0", record, "true")
		testSuccessful(t, "x >= 1", record, "true")
		testSuccessful(t, "x >= 2", record, "false")

		// Test the full matrix of comparisons between all
		// the integer types
		for _, other := range intFields {
			exp := fmt.Sprintf("x == %s", other)
			testSuccessful(t, exp, record, "false")

			exp = fmt.Sprintf("x != %s", other)
			testSuccessful(t, exp, record, "true")

			exp = fmt.Sprintf("x < %s", other)
			testSuccessful(t, exp, record, "false")

			exp = fmt.Sprintf("x <= %s", other)
			testSuccessful(t, exp, record, "false")

			exp = fmt.Sprintf("x > %s", other)
			testSuccessful(t, exp, record, "true")

			exp = fmt.Sprintf("x >= %s", other)
			testSuccessful(t, exp, record, "true")
		}

		// For integer types, test this against other
		// number-ish types: port, time, duration
		if !strings.HasPrefix(typ, "float") {
			record := fmt.Sprintf(
				"{x:%s (%s),p:80 (port=uint16),t:2020-03-09T22:54:12Z,d:16m40s}", one, typ)

			// port
			testSuccessful(t, "x == p", record, "false")
			testSuccessful(t, "p == x", record, "false")
			testSuccessful(t, "x != p", record, "true")
			testSuccessful(t, "p != x", record, "true")
			testSuccessful(t, "x < p", record, "true")
			testSuccessful(t, "p < x", record, "false")
			testSuccessful(t, "x <= p", record, "true")
			testSuccessful(t, "p <= x", record, "false")
			testSuccessful(t, "x > p", record, "false")
			testSuccessful(t, "p > x", record, "true")
			testSuccessful(t, "x >= p", record, "false")
			testSuccessful(t, "p >= x", record, "true")

			// time
			testSuccessful(t, "x == t", record, "false")
			testSuccessful(t, "t == x", record, "false")
			testSuccessful(t, "x != t", record, "true")
			testSuccessful(t, "t != x", record, "true")
			testSuccessful(t, "x < t", record, "true")
			testSuccessful(t, "t < x", record, "false")
			testSuccessful(t, "x <= t", record, "true")
			testSuccessful(t, "t <= x", record, "false")
			testSuccessful(t, "x > t", record, "false")
			testSuccessful(t, "t > x", record, "true")
			testSuccessful(t, "x >= t", record, "false")
			testSuccessful(t, "t >= x", record, "true")

			// duration
			testSuccessful(t, "x == d", record, "false")
			testSuccessful(t, "d == x", record, "false")
			testSuccessful(t, "x != d", record, "true")
			testSuccessful(t, "d != x", record, "true")
			testSuccessful(t, "x < d", record, "true")
			testSuccessful(t, "d < x", record, "false")
			testSuccessful(t, "x <= d", record, "true")
			testSuccessful(t, "d <= x", record, "false")
			testSuccessful(t, "x > d", record, "false")
			testSuccessful(t, "d > x", record, "true")
			testSuccessful(t, "x >= d", record, "false")
			testSuccessful(t, "d >= x", record, "true")
		}

		// Test this against non-numeric types
		record = fmt.Sprintf(
			`{x:%s (%s),s:"hello",i:10.1.1.1,n:10.1.0.0/16} (=0)`, one, typ)

		testSuccessful(t, "x == s", record, "false")
		testSuccessful(t, "x != s", record, "true")
		testSuccessful(t, "x < s", record, "false")
		testSuccessful(t, "x <= s", record, "false")
		testSuccessful(t, "x > s", record, "false")
		testSuccessful(t, "x >= s", record, "false")

		testSuccessful(t, "x == i", record, "false")
		testSuccessful(t, "x != i", record, "true")
		testSuccessful(t, "x < i", record, "false")
		testSuccessful(t, "x <= i", record, "false")
		testSuccessful(t, "x > i", record, "false")
		testSuccessful(t, "x >= i", record, "false")

		testSuccessful(t, "x == n", record, "false")
		testSuccessful(t, "x != n", record, "true")
		testSuccessful(t, "x < n", record, "false")
		testSuccessful(t, "x <= n", record, "false")
		testSuccessful(t, "x > n", record, "false")
		testSuccessful(t, "x >= n", record, "false")
	}

	// Test comparison between signed and unsigned and also
	// floats that cast to different integers.
	const rec2 = "{i:-1,u:18446744073709551615 (uint64),f:-1.} (=0)"

	testSuccessful(t, "i == u", rec2, "false")
	testSuccessful(t, "i != u", rec2, "true")
	testSuccessful(t, "i < u", rec2, "true")
	testSuccessful(t, "i <= u", rec2, "true")
	testSuccessful(t, "i > u", rec2, "false")
	testSuccessful(t, "i >= u", rec2, "false")

	testSuccessful(t, "u == i", rec2, "false")
	testSuccessful(t, "u != i", rec2, "true")
	testSuccessful(t, "u < i", rec2, "false")
	testSuccessful(t, "u <= i", rec2, "false")
	testSuccessful(t, "u > i", rec2, "true")
	testSuccessful(t, "u >= i", rec2, "true")

	testSuccessful(t, "f == u", rec2, "false")
	testSuccessful(t, "f != u", rec2, "true")
	testSuccessful(t, "f < u", rec2, "true")
	testSuccessful(t, "f <= u", rec2, "true")
	testSuccessful(t, "f > u", rec2, "false")
	testSuccessful(t, "f >= u", rec2, "false")

	testSuccessful(t, "u == f", rec2, "false")
	testSuccessful(t, "u != f", rec2, "true")
	testSuccessful(t, "u < f", rec2, "false")
	testSuccessful(t, "u <= f", rec2, "false")
	testSuccessful(t, "u > f", rec2, "true")
	testSuccessful(t, "u >= f", rec2, "true")

	// Test comparisons with unions.
	const rec3 = "{l:1((int64,bytes)),r:2.((string,float64))}"
	testSuccessful(t, "l == r", rec3, "false")
	testSuccessful(t, "l != r", rec3, "true")
	testSuccessful(t, "l < r", rec3, "true")
	testSuccessful(t, "l <= r", rec3, "true")
	testSuccessful(t, "l > r", rec3, "false")
	testSuccessful(t, "l >= r", rec3, "false")
}

func TestCompareNonNumbers(t *testing.T) {
	record := `
{
    b: true,
    s: "hello",
    i: 10.1.1.1,
    p: 443 (port=uint16),
    net: 10.1.0.0/16,
    t: 2020-03-09T22:54:12Z,
    d: 16m40s
} (=0)
`

	// bool
	testSuccessful(t, "b == true", record, "true")
	testSuccessful(t, "b == false", record, "false")
	testSuccessful(t, "b != true", record, "false")
	testSuccessful(t, "b != false", record, "true")

	// string
	testSuccessful(t, `s == "hello"`, record, "true")
	testSuccessful(t, `s != "hello"`, record, "false")
	testSuccessful(t, `s == "world"`, record, "false")
	testSuccessful(t, `s != "world"`, record, "true")

	// ip
	testSuccessful(t, "i == 10.1.1.1", record, "true")
	testSuccessful(t, "i != 10.1.1.1", record, "false")
	testSuccessful(t, "i == 1.1.1.10", record, "false")
	testSuccessful(t, "i != 1.1.1.10", record, "true")
	testSuccessful(t, "i == i", record, "true")

	// port
	testSuccessful(t, "p == 443", record, "true")
	testSuccessful(t, "p != 443", record, "false")

	// net
	testSuccessful(t, "net == 10.1.0.0/16", record, "true")
	testSuccessful(t, "net != 10.1.0.0/16", record, "false")
	testSuccessful(t, "net == 10.1.0.0/24", record, "false")
	testSuccessful(t, "net != 10.1.0.0/24", record, "true")

	// Test comparisons between incompatible types
	allTypes := []struct {
		field string
		typ   string
	}{
		{"b", "bool"},
		{"s", "string"},
		{"i", "ip"},
		{"p", "port"},
		{"net", "net"},
	}

	allOperators := []string{"==", "!=", "<", "<=", ">", ">="}

	for _, t1 := range allTypes {
		for _, t2 := range allTypes {
			if t1 == t2 {
				continue
			}
			for _, op := range allOperators {
				exp := fmt.Sprintf("%s %s %s", t1.field, op, t2.field)
				// XXX we no longer have a way to
				// propagate boolean "warnings"...
				expected := "false"
				if op == "!=" {
					expected = "true"
				}
				testSuccessful(t, exp, record, expected)
			}
		}
	}

	// relative comparisons on strings
	record = `{s:"abc"}`

	testSuccessful(t, `s < "brim"`, record, "true")
	testSuccessful(t, `s < "aaa"`, record, "false")
	testSuccessful(t, `s < "abc"`, record, "false")

	testSuccessful(t, `s > "brim"`, record, "false")
	testSuccessful(t, `s > "aaa"`, record, "true")
	testSuccessful(t, `s > "abc"`, record, "false")

	testSuccessful(t, `s <= "brim"`, record, "true")
	testSuccessful(t, `s <= "aaa"`, record, "false")
	testSuccessful(t, `s <= "abc"`, record, "true")

	testSuccessful(t, `s >= "brim"`, record, "false")
	testSuccessful(t, `s >= "aaa"`, record, "true")
	testSuccessful(t, `s >= "abc"`, record, "true")
}

func TestPattern(t *testing.T) {
	testSuccessful(t, `"abc" == "abc"`, "", "true")
	testSuccessful(t, `"abc" != "abc"`, "", "false")
	testSuccessful(t, "cidr_match(10.0.0.0/8, 10.1.1.1)", "", "true")
	testSuccessful(t, "10.1.1.1 in 192.168.0.0/16", "", "false")
	testSuccessful(t, "!cidr_match(10.0.0.0/8, 10.1.1.1)", "", "false")
	testSuccessful(t, "!(10.1.1.1 in 192.168.0.0/16)", "", "true")
}

func TestIn(t *testing.T) {
	const record = "{a:[1 (int32),2 (int32),3 (int32)] (=0),s:|[4 (int32),5 (int32),6 (int32)]| (=1)} (=2)"

	testSuccessful(t, "1 in a", record, "true")
	testSuccessful(t, "0 in a", record, "false")

	testSuccessful(t, "1 in s", record, "false")
	testSuccessful(t, "4 in s", record, "true")

	testSuccessful(t, `"boo" in a`, record, "false")
	testSuccessful(t, `"boo" in s`, record, "false")
}

func TestArithmetic(t *testing.T) {
	record := "{x:10 (int32),f:2.5} (=0)"

	// Test integer arithmetic
	testSuccessful(t, "100 + 23", record, "123")
	testSuccessful(t, "x + 5", record, "15")
	testSuccessful(t, "5 + x", record, "15")
	testSuccessful(t, "x - 5", record, "5")
	testSuccessful(t, "0 - x", record, "-10")
	testSuccessful(t, "x + 5 - 3", record, "12")
	testSuccessful(t, "x*2", record, "20")
	testSuccessful(t, "5*x*2", record, "100")
	testSuccessful(t, "x/3", record, "3")
	testSuccessful(t, "20/x", record, "2")

	// Test precedence of arithmetic operations
	testSuccessful(t, "x + 1 * 10", record, "20")
	testSuccessful(t, "(x + 1) * 10", record, "110")

	// Test arithmetic with floats
	testSuccessful(t, "f + 1.0", record, "3.5")
	testSuccessful(t, "1.0 + f", record, "3.5")
	testSuccessful(t, "f - 1.0", record, "1.5")
	testSuccessful(t, "0.0 - f", record, "-2.5")
	testSuccessful(t, "f * 1.5", record, "3.75")
	testSuccessful(t, "1.5 * f", record, "3.75")
	testSuccessful(t, "f / 1.25", record, "2.")
	testSuccessful(t, "5.0 / f", record, "2.")

	// Union values are dereferenced when doing arithmetic on them.
	val := "10((int64,string))"
	testSuccessful(t, "this + 1", val, "11")
	testSuccessful(t, "this - 1", val, "9")
	testSuccessful(t, "this * 2", val, "20")
	testSuccessful(t, "this / 2", val, "5")
	testSuccessful(t, "this % 3", val, "1")

	// Test arithmetic with null values
	testSuccessful(t, "null + 1", "", "null(int64)")
	testSuccessful(t, "uint64(1) + null", "", "null(uint64)")
	testSuccessful(t, "null + 1.0", "", "null(float64)")
	testSuccessful(t, "1. - null", "", "null(float64)")
	testSuccessful(t, "uint64(1) * null", "", "null(uint64)")
	testSuccessful(t, "null / 1.", "", "null(float64)")
	testSuccessful(t, "1 / uint64(null)", "", "null(int64)")
	testSuccessful(t, "null % 1", "", "null(int64)")

	// Difference of two times is a duration
	testSuccessful(t, "a - b", "{a:2022-09-22T00:00:01Z,b:2022-09-22T00:00:00Z}", "1s")

	// Difference of time and duration is a time
	testSuccessful(t, "2022-09-22T00:00:01Z - 1h", "", "2022-09-21T23:00:01Z")

	width := func(id int) int {
		switch id {
		case super.IDInt8, super.IDUint8:
			return 8
		case super.IDInt16, super.IDUint16:
			return 16
		case super.IDInt32, super.IDUint32:
			return 32
		case super.IDInt64, super.IDUint64:
			return 64
		}
		panic("width")
	}
	// Test arithmetic between integer types
	intResultDecorator := func(t1, t2 string) string {
		typ1 := super.LookupPrimitive(t1)
		typ2 := super.LookupPrimitive(t2)
		id1 := typ1.ID()
		id2 := typ2.ID()
		sign1 := super.IsSigned(id1)
		sign2 := super.IsSigned(id2)
		sign := true
		if sign1 == sign2 {
			sign = sign1
		}
		w := max(width(id1), width(id2))
		if sign {
			if w == 64 {
				return ""
			}
			return fmt.Sprintf("(int%d)", w)
		}
		return fmt.Sprintf("(uint%d)", w)
	}

	var intTypes = []string{"int8", "uint8", "int16", "uint16", "int32", "uint32", "int64", "uint64"}
	for _, t1 := range intTypes {
		for _, t2 := range intTypes {
			record := fmt.Sprintf("{a:4 (%s),b:2 (%s)} (=0)", t1, t2)
			testSuccessful(t, "a + b", record, "6"+intResultDecorator(t1, t2))
			testSuccessful(t, "b + a", record, "6"+intResultDecorator(t1, t2))
			testSuccessful(t, "a - b", record, "2"+intResultDecorator(t1, t2))
			testSuccessful(t, "a * b", record, "8"+intResultDecorator(t1, t2))
			testSuccessful(t, "b * a", record, "8"+intResultDecorator(t1, t2))
			testSuccessful(t, "a / b", record, "2"+intResultDecorator(t1, t2))
			testSuccessful(t, "b / a", record, "0"+intResultDecorator(t1, t2))
		}

		// Test arithmetic mixing float + int
		record = fmt.Sprintf("{x:10 (%s),f:2.5} (=0)", t1)
		testSuccessful(t, "f + 5", record, "7.5")
		testSuccessful(t, "5 + f", record, "7.5")
		testSuccessful(t, "f + x", record, "12.5")
		testSuccessful(t, "x + f", record, "12.5")
		testSuccessful(t, "x - f", record, "7.5")
		testSuccessful(t, "f - x", record, "-7.5")
		testSuccessful(t, "x*f", record, "25.")
		testSuccessful(t, "f*x", record, "25.")
		testSuccessful(t, "x/f", record, "4.")
		testSuccessful(t, "f/x", record, "0.25")
	}
	// Test string concatenation
	testSuccessful(t, `"hello" + " world"`, record, `"hello world"`)

	// Test string arithmetic other than + fails
	testSuccessful(t, `"hello" - " world"`, record, `error("type string incompatible with '-' operator")`)
	testSuccessful(t, `"hello" * " world"`, record, `error("type string incompatible with '*' operator")`)
	testSuccessful(t, `"hello" / " world"`, record, `error("type string incompatible with '/' operator")`)

	// Test that addition fails on an unsupported type
	testSuccessful(t, "10.1.1.1 + 1", record, `error("incompatible types")`)
	testSuccessful(t, "10.1.1.1 + 3.14159", record, `error("incompatible types")`)
	testSuccessful(t, `10.1.1.1 + "foo"`, record, `error("incompatible types")`)
}

func TestArrayIndex(t *testing.T) {
	const record = `{x:[1,2,3],i:2 (uint16)} (=0)`

	testSuccessful(t, "x[1]", record, "1")
	testSuccessful(t, "x[2]", record, "2")
	testSuccessful(t, "x[3]", record, "3")
	testSuccessful(t, "x[i]", record, "2")
	testSuccessful(t, "i+1", record, "3")
	testSuccessful(t, "x[i+1]", record, "3")
}

func TestFieldReference(t *testing.T) {
	const record = `{rec:{i:5 (int32),s:"boo",f:6.1} (=0)} (=1)`

	testSuccessful(t, "rec.i", record, "5(int32)")
	testSuccessful(t, "rec.s", record, `"boo"`)
	testSuccessful(t, "rec.f", record, "6.1")
}

func TestConditional(t *testing.T) {
	const record = "{x:1}"

	testSuccessful(t, `x == 0 ? "zero" : "not zero"`, record, `"not zero"`)
	testSuccessful(t, `x == 1 ? "one" : "not one"`, record, `"one"`)
	testSuccessful(t, `x ? "x" : "not x"`, record, `error({message:"?-operator: bool predicate required",on:1})`)

	// Ensure that the unevaluated clause doesn't generate errors
	// (field y doesn't exist but it shouldn't be evaluated)
	testSuccessful(t, "x == 0 ? y : x", record, "1")
	testSuccessful(t, "x != 0 ? x : y", record, "1")
}

func TestCasts(t *testing.T) {
	// Test casts to byte
	testSuccessful(t, "uint8(10)", "", "10(uint8)")
	testSuccessful(t, "uint8(-1)", "", `error({message:"cannot cast to uint8",on:-1})`)
	testSuccessful(t, "uint8(300)", "", `error({message:"cannot cast to uint8",on:300})`)
	testSuccessful(t, `uint8("foo")`, "", `error({message:"cannot cast to uint8",on:"foo"})`)
	testSuccessful(t, `uint8("-1")`, "", `error({message:"cannot cast to uint8",on:"-1"})`)
	testSuccessful(t, "uint8(258.)", "", `error({message:"cannot cast to uint8",on:258.})`)
	testSuccessful(t, `uint8("255")`, "", `255(uint8)`)

	// Test casts to int16
	testSuccessful(t, "int16(10)", "", "10(int16)")
	testSuccessful(t, "int16(-33000)", "", `error({message:"cannot cast to int16",on:-33000})`)
	testSuccessful(t, "int16(33000)", "", `error({message:"cannot cast to int16",on:33000})`)
	testSuccessful(t, `int16("foo")`, "", `error({message:"cannot cast to int16",on:"foo"})`)

	// Test casts to uint16
	testSuccessful(t, "uint16(10)", "", "10(uint16)")
	testSuccessful(t, "uint16(-1)", "", `error({message:"cannot cast to uint16",on:-1})`)
	testSuccessful(t, "uint16(66000)", "", `error({message:"cannot cast to uint16",on:66000})`)
	testSuccessful(t, `uint16("foo")`, "", `error({message:"cannot cast to uint16",on:"foo"})`)

	// Test casts to int32
	testSuccessful(t, "int32(10)", "", "10(int32)")
	testSuccessful(t, "int32(-2200000000)", "", `error({message:"cannot cast to int32",on:-2200000000})`)
	testSuccessful(t, "int32(2200000000)", "", `error({message:"cannot cast to int32",on:2200000000})`)
	testSuccessful(t, `int32("foo")`, "", `error({message:"cannot cast to int32",on:"foo"})`)

	// Test casts to uint32
	testSuccessful(t, "uint32(10)", "", "10(uint32)")
	testSuccessful(t, "uint32(-1)", "", `error({message:"cannot cast to uint32",on:-1})`)
	testSuccessful(t, "uint32(4300000000)", "", `error({message:"cannot cast to uint32",on:4300000000})`)
	testSuccessful(t, "uint32(-4.3e9)", "", `error({message:"cannot cast to uint32",on:-4300000000.})`)
	testSuccessful(t, `uint32("foo")`, "", `error({message:"cannot cast to uint32",on:"foo"})`)

	// Test cast to int64
	testSuccessful(t, "int64(this)", "10000000000000000000(uint64)", `error({message:"cannot cast to int64",on:10000000000000000000(uint64)})(error({message:string,on:uint64}))`)
	testSuccessful(t, "int64(this)", "1e+19", `error({message:"cannot cast to int64",on:1e+19})`)
	testSuccessful(t, `int64("10000000000000000000")`, "", `error({message:"cannot cast to int64",on:"10000000000000000000"})`)

	// Test casts to uint64
	testSuccessful(t, "uint64(10)", "", "10(uint64)")
	testSuccessful(t, "uint64(-1)", "", `error({message:"cannot cast to uint64",on:-1})`)
	testSuccessful(t, `uint64("foo")`, "", `error({message:"cannot cast to uint64",on:"foo"})`)
	testSuccessful(t, `uint64(+Inf)`, "", `error({message:"cannot cast to uint64",on:+Inf})`)

	// Test casts to float16
	testSuccessful(t, "float16(10)", "", "10.(float16)")
	testSuccessful(t, `float16("foo")`, "", `error({message:"cannot cast to float16",on:"foo"})`)

	// Test casts to float32
	testSuccessful(t, "float32(10)", "", "10.(float32)")
	testSuccessful(t, `float32("foo")`, "", `error({message:"cannot cast to float32",on:"foo"})`)

	// Test casts to float64
	testSuccessful(t, "float64(10)", "", "10.")
	testSuccessful(t, `float64("foo")`, "", `error({message:"cannot cast to float64",on:"foo"})`)

	// Test casts to ip
	testSuccessful(t, `ip("1.2.3.4")`, "", "1.2.3.4")
	testSuccessful(t, "ip(1234)", "", `error({message:"cannot cast to ip",on:1234})`)
	testSuccessful(t, `ip("not an address")`, "", `error({message:"cannot cast to ip",on:"not an address"})`)

	// Test casts to net
	testSuccessful(t, `net("1.2.3.0/24")`, "", "1.2.3.0/24")
	testSuccessful(t, "net(1234)", "", `error({message:"cannot cast to net",on:1234})`)
	testSuccessful(t, `net("not an address")`, "", `error({message:"cannot cast to net",on:"not an address"})`)
	testSuccessful(t, `net(1.2.3.4)`, "", `error({message:"cannot cast to net",on:1.2.3.4})`)

	// Test casts to time
	testSuccessful(t, "time(float16(65504))", "", "1970-01-01T00:00:00.000065504Z")
	// float32 lacks sufficient precision to represent this time exactly.
	testSuccessful(t, "time(float32(1589126400000000000))", "", "2020-05-10T15:59:14.647908352Z")
	testSuccessful(t, "time(float64(1589126400000000000))", "", "2020-05-10T16:00:00Z")
	testSuccessful(t, "time(1589126400000000000)", "", "2020-05-10T16:00:00Z")
	testSuccessful(t, `time("1589126400000000000")`, "", "2020-05-10T16:00:00Z")

	testSuccessful(t, "string(1.2)", "", `"1.2"`)
	testSuccessful(t, "string(5)", "", `"5"`)
	testSuccessful(t, "string(this)", "5(uint8)", `"5"`)
	testSuccessful(t, "string(this)", "5.5(float16)", `"5.5"`)
	testSuccessful(t, "string(1.2.3.4)", "", `"1.2.3.4"`)
	testSuccessful(t, `int64("1")`, "", "1")
	testSuccessful(t, `int64("-1")`, "", "-1")
	testSuccessful(t, `float16("5.5")`, "", "5.5(float16)")
	testSuccessful(t, `float32("5.5")`, "", "5.5(float32)")
	testSuccessful(t, `float64("5.5")`, "", "5.5")
	testSuccessful(t, `ip("1.2.3.4")`, "", "1.2.3.4")

	testSuccessful(t, "ip(1)", "", `error({message:"cannot cast to ip",on:1})`)
	testSuccessful(t, `int64("abc")`, "", `error({message:"cannot cast to int64",on:"abc"})`)
	testSuccessful(t, `float16("abc")`, "", `error({message:"cannot cast to float16",on:"abc"})`)
	testSuccessful(t, `float32("abc")`, "", `error({message:"cannot cast to float32",on:"abc"})`)
	testSuccessful(t, `float64("abc")`, "", `error({message:"cannot cast to float64",on:"abc"})`)
	testSuccessful(t, `ip("abc")`, "", `error({message:"cannot cast to ip",on:"abc"})`)
}
