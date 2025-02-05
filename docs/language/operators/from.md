### Operator

&emsp; **from** &mdash; source data from pools, files, or URIs

### Synopsis

```
from <pool>[@<commitish>]
from <pattern>
file <path> [format <format>]
get <uri> [format <format>]
from (
   pool <pool>[@<commitish>] [ => <branch> ]
   pool <pattern>
   file <path> [format <format>] [ => <branch> ]
   get <uri> [format <format>] [ => <branch> ]
   pass
   ...
)
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
from PoolOne | fork (
  => op1 | op2 | ...
  => op1 | op2 | ...
) | merge ts | ...
```

Or multiple pools can be accessed and, for example, joined:
```
from (
  pool PoolOne => op1 | op2 | ...
  pool PoolTwo => op1 | op2 | ...
) | join on key=key | ...
```

Similarly, data can be routed to different pipeline branches with replication
using the [`switch` operator](switch.md):
```
from ... | switch color (
  case "red" => op1 | op2 | ...
  case "blue" => op1 | op2 | ...
  default => op1 | op2 | ...
) | ...
```

### Input Data

Examples below below assume the existence of the SuperDB lake created and populated
by the following commands:

```mdtest-command
export SUPER_DB_LAKE=example
super db -q init
super db -q create -orderby flip:desc coinflips
echo '{flip:1,result:"heads"} {flip:2,result:"tails"}' |
  super db load -q -use coinflips -
super db branch -q -use coinflips trial
echo '{flip:3,result:"heads"}' | super db load -q -use coinflips@trial -
super db -q create numbers
echo '{number:1,word:"one"} {number:2,word:"two"} {number:3,word:"three"}' |
  super db load -q -use numbers -
super db query -f text '
  from :branches
  | yield pool.name + "@" + branch.name
  | sort'
```

The lake then contains the two pools:

```mdtest-output
coinflips@main
coinflips@trial
numbers@main
```

The following file `hello.jsup` is also used.

```mdtest-input hello.jsup
{greeting:"hello world!"}
```

### Examples

_Source structured data from a local file_

```mdtest-command
super -z -c 'file hello.jsup | yield greeting'
```
=>
```mdtest-output
"hello world!"
```

_Source data from a local file, but in line format_
```mdtest-command
super -z -c 'file hello.jsup format line'
```
=>
```mdtest-output
"{greeting:\"hello world!\"}"
```

_Source structured data from a URI_
```
super -z -c 'get https://raw.githubusercontent.com/brimdata/zui-insiders/main/package.json
       | yield productName'
```
=>
```
"Zui - Insiders"
```

_Source data from the `main` branch of a pool_
```mdtest-command
super db -lake example query -z 'from coinflips'
```
=>
```mdtest-output
{flip:2,result:"tails"}
{flip:1,result:"heads"}
```

_Source data from a specific branch of a pool_
```mdtest-command
super db -lake example query -z 'from coinflips@trial'
```
=>
```mdtest-output
{flip:3,result:"heads"}
{flip:2,result:"tails"}
{flip:1,result:"heads"}
```

_Count the number of values in the `main` branch of all pools_
```mdtest-command
super db -lake example query -f text 'from * | count()'
```
=>
```mdtest-output
5
```
_Join the data from multiple pools_
```mdtest-command
super db -lake example query -z '
  from coinflips | sort flip
  | join (
    from numbers | sort number
  ) on flip=number word'
```
=>
```mdtest-output
{flip:1,result:"heads",word:"one"}
{flip:2,result:"tails",word:"two"}
```

_Use `pass` to combine our join output with data from yet another source_
```mdtest-command
super db -lake example query -z '
  from coinflips | sort flip
  | join (
    from numbers | sort number
  ) on flip=number word
  | from (
    pass
    pool coinflips@trial =>
      c:=count()
      | yield f"There were {int64(c)} flips"
  ) | sort this'
```
=>
```mdtest-output
"There were 3 flips"
{flip:1,result:"heads",word:"one"}
{flip:2,result:"tails",word:"two"}
```
