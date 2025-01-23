### Function

&emsp; **grep** &mdash; search strings inside of values

### Synopsis

```
grep(<pattern> [, e: any]) -> bool
```

### Description

The _grep_ function searches all of the strings in its input value `e`
(or `this` if `e` is not given)
 using the `<pattern>` argument, which can be a
[regular expression](../search-expressions.md#regular-expressions),
[glob pattern](../search-expressions.md#globs), or string.
If the pattern matches for any string, then the result is `true`.  Otherwise, it is `false`.

{{% tip "Note" %}}

String matches are case insensitive while regular expression
and glob matches are case sensitive.  In a forthcoming release, case sensitivity
will be a expressible for all three pattern types.

{{% /tip %}}

The entire input value is traversed:
* for records, each field name is traversed and each field value is traversed or descended
if a complex type,
* for arrays and sets, each element is traversed or descended if a complex type, and
* for maps, each key and value is traversed or descended if a complex type.

### Examples

_Reach into nested records_
```mdtest-spq
# spq
grep("baz")
# input
{foo:10}
{bar:{s:"baz"}}
# expected output
{bar:{s:"baz"}}
```

_It only matches string fields_
```mdtest-spq
# spq
grep("10")
# input
{foo:10}
{bar:{s:"baz"}}
# expected output
```

_Match a field name_
```mdtest-spq
# spq
grep("foo")
# input
{foo:10}
{bar:{s:"baz"}}
# expected output
{foo:10}
```

_Regular expression_
```mdtest-spq
# spq
grep(/foo|baz/)
# input
{foo:10}
{bar:{s:"baz"}}
# expected output
{foo:10}
{bar:{s:"baz"}}
```

_Glob with a second argument_
```mdtest-spq
# spq
grep(b*, s)
# input
{s:"bar"}
{s:"foo"}
{s:"baz"}
{t:"baz"}
# expected output
{s:"bar"}
{s:"baz"}
```
