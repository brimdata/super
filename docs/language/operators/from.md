### Operator

&emsp; **from** &mdash; source data from pools, files, or URIs

### Synopsis

```
from <pool>[@<commitish>]
from <pattern>
```
### Description

The `from` operator identifies one or more data sources and transmits
their data to its output.  A data source can be
* the name of a data pool in a SuperDB lake, with optional [commitish](../../commands/super-db.md#commitish);
* the names of multiple data pools, expressed as a [regular expression](../search-expressions.md#regular-expressions) or [glob](../search-expressions.md#globs) pattern;
* a path to a file;
* an HTTP, HTTPS, or S3 URI; or
* the [`pass` operator](pass.md), to treat the upstream pipeline branch as a source.

{{% tip "Note" %}}

File paths and URIs may be followed by an optional [format](../../commands/super.md#input-formats) specifier.

{{% /tip %}}

Sourcing data from pools is only possible when querying a lake, such as
via the [`super db` command](../../commands/super-db.md) or
[SuperDB lake API](../../lake/api.md). Sourcing data from files is only possible
with the [`super` command](../../commands/super.md).

When a single pool name is specified without `@`-referencing a commit or ID, or
when using a pool pattern, the tip of the `main` branch of each pool is
accessed.

In the first four forms, a single source is connected to a single output.
In the fifth form, multiple sources are accessed in parallel and may be
[joined](join.md), [combined](combine.md), or [merged](merge.md).

A pipeline can be split with the [`fork` operator](fork.md) as in
```
from PoolOne | fork
  ( op1 | op2 | ... )
  ( op1 | op2 | ... )
| merge ts | ...
```

Or multiple pools can be accessed and, for example, joined:
```
fork
  ( from PoolOne | op1 | op2 | ... )
  ( from PoolTwo | op1 | op2 | ... )
| join on key=key | ...
```

Similarly, data can be routed to different pipeline branches with replication
using the [`switch` operator](switch.md):
```
from ... | switch color
  case "red" ( op1 | op2 | ... )
  case "blue" ( op1 | op2 | ... )
  default ( op1 | op2 | ... )
| ...
```

### Input Data

Examples below below assume the existence of the SuperDB lake created and populated
by the following commands:

```mdtest-command
export SUPER_DB=example
super db -q init
super db -q create -orderby flip:desc coinflips
echo '{flip:1,result:"heads"} {flip:2,result:"tails"}' |
  super db load -q -use coinflips -
super db branch -q -use coinflips trial
echo '{flip:3,result:"heads"}' | super db load -q -use coinflips@trial -
super db -q create numbers
echo '{number:1,word:"one"} {number:2,word:"two"} {number:3,word:"three"}' |
  super db load -q -use numbers -
super db -f text -c '
  from :branches
  | values pool.name + "@" + branch.name
  | sort'
```

The lake then contains the two pools:

```mdtest-output
coinflips@main
coinflips@trial
numbers@main
```

The following file `hello.sup` is also used.

```mdtest-input hello.sup
{greeting:"hello world!"}
```

### Examples

_Source structured data from a local file_

```mdtest-command
super -s -c 'from hello.sup | values greeting'
```
=>
```mdtest-output
"hello world!"
```

_Source data from a local file, but in line format_
```mdtest-command
super -s -c 'from hello.sup (format line)'
```
=>
```mdtest-output
"{greeting:\"hello world!\"}"
```

_Source structured data from a URI_
```
super -s -c 'get https://raw.githubusercontent.com/brimdata/zui-insiders/main/package.json
       | values productName'
```
=>
```
"Zui - Insiders"
```

_Source data from the `main` branch of a pool_
```mdtest-command
super db -db example -s -c 'from coinflips'
```
=>
```mdtest-output
{flip:2,result:"tails"}
{flip:1,result:"heads"}
```

_Source data from a specific branch of a pool_
```mdtest-command
super db -db example -s -c 'from coinflips@trial'
```
=>
```mdtest-output
{flip:3,result:"heads"}
{flip:2,result:"tails"}
{flip:1,result:"heads"}
```

_Count the number of values in the `main` branch of all pools_
```mdtest-command
super db -db example -f text -c 'from * | count()'
```
=>
```mdtest-output
5
```
_Join the data from multiple pools_
```mdtest-command
super db -db example -s -c '
  from coinflips | sort flip
  | join (
    from numbers | sort number
  ) on left.flip=right.number
  | values {...left, word:right.word}'
```
=>
```mdtest-output
{flip:1,result:"heads",word:"one"}
{flip:2,result:"tails",word:"two"}
```

_Use `pass` to combine our join output with data from yet another source_
```mdtest-command
super db -db example -s -c '
  from coinflips | sort flip
  | join (
    from numbers | sort number
  ) on left.flip=right.number
  | values {...left, word:right.word}
  | fork 
    ( pass )
    ( from coinflips@trial 
      | c:=count()
      | values f"There were {int64(c)} flips" )
  | sort this'
```
=>
```mdtest-output
"There were 3 flips"
{flip:1,result:"heads",word:"one"}
{flip:2,result:"tails",word:"two"}
```
