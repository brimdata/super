### Operator

&emsp; **from** &mdash; source data from databases, files, URIs, or subqueries

### Synopsis

```
from <file> [ ( format <name> ) ]
from <pool>
from <uri> [ ( format <name> method <id> header <expr> body <string> ) ]
from <glob> [ ( format <name> ) ]
from <regexp>
from eval(<expr>) [ ( format <name> method <id> header <expr> body <string> ) ]
```

### Description

The `from` keyword signals one of two overlapping forms of operators:
* a pipe operator with dataflow scoping as described here, or
* a [SQL clause](../sql/from.md) with relational scoping.

The `from` operator identifies one or more data sources and transmits
their data to its output.

When multiple sources are identified, the data may be read in parallel 
and interleaved in an undefined order.  The order of the data within a file,
URI, or a sorted pool is preserved at the output of `from`.

Optional arguments to `from` may be appended as a parenthesized concatenation
of arguments.

The format of each data source is automatically but detected using heuristics.
To manually specify the format of a source and override the autodetection heuristic,
a format argument may be appended as an argument and has the form
```
format <name>
```
where `<name>` is the name of a supported
[serialization format](../../commands/super.md#input-formats).

#### Files

When running without a database, a string argument or
unquoted path identifier that does not match a URI is interpreted
as a path to a file.

XXX define file identifier, or path identifier?

Files can also be referenced using a
[glob](../search-expressions.md#globs) pattern.

E.g., 
```
from "file.sup"
from file.json (format json)
from file*.parquet
```

#### Pools

When running with a database, a string argument or
unquoted path identifier that does not match a URI is interpreted
as the name of a database pool.

Sourcing data from pools is only possible when querying a database, such as
via the [`super db` command](../../command/super-db.md) or
[SuperDB API](../../database/api.md)

The names of multiple data pools may also be expressed as a
[regular expression](../search-expressions.md#regular-expressions) or
[glob](../search-expressions.md#globs) pattern.

The reference string for a pool may also include
[commitish](../../commands/super-db.md#commitish)
to read from a specific commit in the pool's commit history.

When a single pool name is specified without `@`-referencing a commit, or
when using a pool pattern, the tip of the `main` branch of each pool is
accessed.

The format argument is not valid with a database source.

XXX :metadata at database level and pool level

#### URIs

Data sources identified by URIs can be accessed both when running with a database
and without.

URIs must begin with `http:`, `https:`, or `s3:`.

A URI may be unquoted or quoted as a string, e.g., if it contains special characters.

A format argument may be appended to a URI reference.

XXX documnt rule for unquoted URI

XXX take "merge" and "combine" out of docs and document them in pipeline dataflow

### Examples


A pipeline can be split with the [`fork` operator](fork.md) as in
```
from PoolOne
| fork
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
from ...
| switch color
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

---

_Source structured data from a local file_

```mdtest-command
cat '{greeting:"hello world!"}' > hello.sup
super -s -c 'from hello.sup | values greeting'
```
=>
```mdtest-output
"hello world!"
```

---

_Source data from a local file, but in line format_
```mdtest-command
super -s -c 'from hello.sup format line'
```
=>
```mdtest-output
"{greeting:\"hello world!\"}"
```

---

_Source structured data from a URI_
```
super -s -c 'from https://raw.githubusercontent.com/brimdata/zui-insiders/main/package.json
       | values productName'
```
=>
```
"Zui - Insiders"
```

---

_Source data from the `main` branch of a pool_
```mdtest-command
super db -db example -s -c 'from coinflips'
```
=>
```mdtest-output
{flip:2,result:"tails"}
{flip:1,result:"heads"}
```

---

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

---

_Count the number of values in the `main` branch of all pools_
```mdtest-command
super db -db example -f text -c 'from * | count()'
```
=>
```mdtest-output
5
```

---

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

---

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
