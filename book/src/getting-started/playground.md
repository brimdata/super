## Playground

If you have `super` installed, a common pattern for experimentation is to
"echo" some input to the `super -c` command, e.g.,
```
echo <values> | super -c <query> -
```
But you can also experiment with SuperDB using the browser-embedded
playground.  The `super` binary has been
[compiled into Web assembly](https://github.com/brimdata/superdb-wasm)
and executes a `super -c <query> -` command like this:
```mdtest-spq
# spq
SELECT upper(message) AS out
# input
{id:0,message:"Hello"}
{id:1,message:"Goodbye"}
# expected output
{out:"HELLO"}
{out:"GOODBYE"}
```
The QUERY and INPUT panes are editable.  So go ahead and experiment.
Try changing `upper` to `lower` in the query text
and you should get this alternative output in the RESULT panel above:
```
{out:"hello"}
{out:"goodbye"}
```
The input in the playground examples are generally formatted as
[SUP](../formats/sup.md) but the `super` playground command autodetects
the format, so feel free to experiment with other text formats like CSV or JSON.
For example, if you change the input above to
```
id,message
0,"Hello"
1,"Goodbye"
```
`super` will detect this as CSV and you will get the same result.

### Copy to CLI

If you want to transfer a working playground query to your shell, just
click on CLI tab and you will see text suitable for pasting into a
`bash`, `zsh`, etc.

> Note that there is no special quoting for the shell so you may run
> into problems if you mix single and double quotes in your query.

### Examples

To explore a broad range of SuperSQL functionality,
try browsing the documentation for
[pipe operators](../super-sql/operators/intro.md) or
[functions](../super-sql/functions/intro.md).
Each operator and function has a section of examples
with playgrounds where you can edit
the example queries and inputs to explore how SuperSQL works.
The [tutorials section](../tutorials/intro.md)
also has many playground examples.

Here are a few examples to get going.

---

**_Hello, world_**

```mdtest-command
super -s -c "SELECT VALUE 'hello, world'"
```
produces this SUP output
```mdtest-output
"hello, world"
```

---

**_Some values of available [data types](../super-sql/types/intro.md)_**

```mdtest-spq
# spq
SELECT VALUE in
# input
{in:1}
{in:1.5}
{in:[1,"foo"]}
{in:|["apple","banana"]|}
# expected output
1
1.5
[1,"foo"]
|["apple","banana"]|
```

---

**_The types of various data_**

```mdtest-spq
# spq
SELECT VALUE typeof(in)
# input
{in:1}
{in:1.5}
{in:[1,"foo"]}
{in:|["apple","banana"]|}
# expected output
<int64>
<float64>
<[int64|string]>
<|[string]|>
```

---

**_A simple [aggregation](../super-sql/operators/aggregate.md)_**
```mdtest-spq
# spq
sum(val) by key | sort key
# input
{key:"foo",val:1}
{key:"bar",val:2}
{key:"foo",val:3}
# expected output
{key:"bar",sum:2}
{key:"foo",sum:4}
```

---

**_Read CSV input and [cast](../super-sql/functions/types/cast.md) a to an integer from default float_**
```mdtest-spq
# spq
a:=a::int64
# input
a,b
1,foo
2,bar
# expected output
{a:1,b:"foo"}
{a:2,b:"bar"}
```

---

**_Read JSON input and cast to an integer from default float_**
```mdtest-spq
# spq
a:=a::int64
# input
{"a":1,"b":"foo"}
{"a":2,"b":"bar"}
# expected output
{a:1,b:"foo"}
{a:2,b:"bar"}
```

---

**_Make a schema-rigid Parquet file using fuse, then output the Parquet file
as [SUP](../formats/sup.md)_**
```mdtest-command
echo '{a:1}{a:2}{b:3}' | super -f parquet -o tmp.parquet -c fuse -
super -s tmp.parquet
```
produces
```mdtest-output
{a:1,b:null::int64}
{a:2,b:null::int64}
{a:null::int64,b:3}
```
