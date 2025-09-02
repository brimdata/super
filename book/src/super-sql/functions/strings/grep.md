### Function

&emsp; **grep** &mdash; search strings inside of values

### Synopsis

```
grep(re: string e: any) -> bool
```

### Description

The `grep` function searches all of the strings in its input value `e`
 using the `re` argument, which is a
[regular expression](../../patterns.md#regular-expression).
If the pattern matches for any string, then the result is `true`.  Otherwise, it is `false`.

> _String matches are case insensitive while regular expression
> and glob matches are case sensitive.  In a forthcoming release, case sensitivity
> will be expressible for all three pattern types._

The entire input value is traversed:
* for records, each field name is traversed and each field value is traversed or descended
if a complex type,
* for arrays and sets, each element is traversed or descended if a complex type, and
* for maps, each key and value is traversed or descended if a complex type.

### Examples

---

_Reach into nested records_

```mdtest-spq
# spq
grep("baz", this)
# input
{foo:10}
{bar:{s:"baz"}}
# expected output
{bar:{s:"baz"}}
```

---

_It only matches string fields_

```mdtest-spq
# spq
grep("10", this)
# input
{foo:10}
{bar:{s:"baz"}}
# expected output
```

---

_Match a field name_

```mdtest-spq
# spq
grep("foo", this)
# input
{foo:10}
{bar:{s:"baz"}}
# expected output
{foo:10}
```

---

_Regular expression_

```mdtest-spq
# spq
grep("foo|baz", this)
# input
{foo:10}
{bar:{s:"baz"}}
# expected output
{foo:10}
{bar:{s:"baz"}}
```

---

_Regular expression with a non-this argument_

```mdtest-spq
# spq
grep('b.*', s)
# input
{s:"bar"}
{s:"foo"}
{s:"baz"}
{t:"baz"}
# expected output
{s:"bar"}
{s:"baz"}
```
