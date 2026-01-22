### Operator

[✅](../intro.md#data-order)[🎲](../intro.md#data-order)&emsp; **from** &mdash; source data from databases, files, or URLs

### Synopsis

```
from <file> [ ( format <fmt> ) ]
from <pool> [@<commit>]
from <url> [ ( format <fmt> method <method> headers <expr> body <string> ) ]
from eval(<expr>) [ ( format <fmt> method <id> headers <expr> body <string> ) ]
```

### Description

The `from` operator identifies one or more sources of data as input to
a query and transmits that data to its output.

It has two forms:
* a `from` pipe operator with [pipe scoping](../intro.md#pipe-scoping) as described here, or
* a SQL [`FROM`](../sql/from.md) clause with
  [relational scoping](../intro.md#relational-scoping).

As a pipe operator,
`from` preserves the order of the data within a file,
URL, or a sorted pool but when multiple sources are identified,
the data may be read in parallel and interleaved in an undefined order.

Optional arguments to `from` may be appended as a parenthesized concatenation
of arguments.

When reading from sources external to a [database](../../command/db.md) (e.g., URLs or files),
the format of each data source is automatically detected using heuristics.
To manually specify the format of a source and override the autodetection heuristic,
a format argument may be appended as an argument and has the form
```
format <fmt>
```
where `<fmt>` is the name of a supported
[serialization format](../../command/super.md#supported-formats) and is
parsed as a [text entity](../queries.md#text-entity).

When `from` references a file or URL entity whose name ends in a
[well-known extension](../../command/super.md#supported-formats)
(e.g., `.json`, `.sup`, etc.), auto-detection is disabled and the
format is implied by the extension name.

#### File-System Operation

When running detached from a database, the target of `from`
is either a
[text entity](../queries.md#text-entity)
or a file system [glob](../queries.md#glob).

If a text entity is parseable as an HTTP or HTTPS URL,
then the target is presumed to be a [URL](#url) and is processed
accordingly.  Otherwise, the target is assumed to be a file
in the file system whose path is relative to the directory
in which the `super` command is running.

If the target is a glob, then the glob is expanded and the files
are processed in an undefined order.  Any operator arguments specified
after a glob target are applied to all of the matched files.

Here are a few examples illustrating file references:
```
from "file.sup"
from file.json
from file*.parq (format parquet)
```

#### Database Operation

When running attached to a database (i.e., using `super db`),
the target of `from` is either a
[text entity](../queries.md#text-entity)
or a [regular expression](../queries.md#regular-expression)
or [glob](../queries.md#glob) that matches pool names.

If a text entity is parseable as an HTTP or HTTPS URL,
then the target is presumed to be a [URL](#url) and is processed
accordingly.  Otherwise, the target is assumed to be the name
of a pool in the attached database.

Local files are not accessible when attached to a database.

Note that pool names and file names have similar syntax in `from` but
their use is disambiguated by the presence or absence of an attached
database.

When multiple data pools are referenced with a glob or regular expression,
they are scanned in an undefined order.

The reference string for a pool may also be appended with an `@`-style
[commitish](../../database/intro.md#commitish), which specifies that
data is sourced from a specific commit in a pool's commit history.

When a single pool name is specified without an `@` reference, or
when using a glob or regular expression, the tip of the `main`
branch of each pool is accessed.

The format argument is not valid with a database source.

>[!NOTE]
> Metadata from database pools also may be sourced using `from`.
> This will be documented in a future release of SuperDB.

#### URL

Data sources identified by URLs can be accessed either when attached
or detached from a database.

When the `<url>` argument begins with `http:` or `https:`
and has the form of a valid URL, then the source is fetched remotely using the
indicated protocol.

As a [text entity](../queries.md#text-entity), typical URLs need not be quoted
though URLs with special characters must be quoted.

A format argument may be appended to a URL reference.

Other valid operator arguments control the body and headers of the HTTP request
that implement the data retrieval and include:
* method `<method>`
* headers `<expr>`
* body `<string>`

where

* `<method>` is one of `GET`, `PUT`, `POST`, or `DELETE`,
* `<expr>` is a [record expression](../types/record.md) that defines the names and values
to be included as HTTP header options, and
* `<body>` is a [text-entity](../queries.md#text-entity) string
to be included as the body of the HTTP request.

Currently, the headers expression must evaluate to a compile-time constant though this
may change to allow dynamic computation in a future version of SuperSQL.
Each field of this record must either be a string or (to specify a
header option appearing multiple times with different values)
an array or set of strings.

#### Expression

The `eval()` form of `from` provides a means to read data programmatically from
sources based on the `<expr>` argument to `eval`, which should return
a value of type [`string`](../types/string.md).
In this case, `from` reads values from its parent, applies `<expr>` to each
value, and interprets the string result as a target to be processed.

Each string value is interpreted as a from target and must be a file path
(when running detached from a database), a pool name (when attached to a database),
or a URL forming a sequence of targets which are read and output by the
`from` operator in the order encountered.

#### Combining Data

To combine data from multiple sources using pipe operators, `from` may be
used in combination with other operators like [`fork`](fork.md) and [`join`](join.md).

For example, multiple pools can be accessed in parallel
and combined in undefined order:
```
fork
  ( from PoolOne | op1 | op2 | ... )
  ( from PoolTwo | op1 | op2 | ... )
| ...
```
or joined according to a join condition:
```
fork
  ( from PoolOne | op1 | op2 | ... )
  ( from PoolTwo | op1 | op2 | ... )
| join as {left,right} on left.key=right.key
| ...
```
Alternatively, the right-hand leg of the join may be written as a subquery
of join:
```
from PoolOne | op1 | op2 | ...
| join ( from PoolTwo | op1 | op2 | ... )
    as {left,right} on left.key=right.key
| ...
```

### File Examples

---

_Source structured data from a local file_

```mdtest-command
echo '{greeting:"hello world!"}' > hello.sup
super -s -c 'from hello.sup | values greeting'
```
=>
```mdtest-output
"hello world!"
```

---

_Source data from a local file, but in "line" format_
```mdtest-command
super -s -c 'from hello.sup (format line)'
```
=>
```mdtest-output
"{greeting:\"hello world!\"}"
```

### HTTP Example

---

_Source data from a URL_
```
super -s -c 'from https://raw.githubusercontent.com/brimdata/super/main/package.json
       | values name'
```
=>
```
"super"
```

---

### Database Examples

The remaining examples below assume the existence of the SuperDB database
created and populated by the following commands:

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
super db -f line -c '
  from :branches
  | values pool.name || "@" || branch.name
  | sort'
```

The database then contains the two pools and three branches:

```mdtest-output
coinflips@main
coinflips@trial
numbers@main
```

The following file `hello.sup` is also used.

```mdtest-input hello.sup
{greeting:"hello world!"}
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
super db -db example -s -c 'from * | count()'
```
=>
```mdtest-output
5
```

---

_Join the data from multiple pools_

```mdtest-command
super db -db example -s -c '
  from coinflips
  | join ( from numbers ) on left.flip=right.number
  | values {...left, word:right.word}
  | sort'
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
  from coinflips
  | join ( from numbers ) on left.flip=right.number
  | values {...left, word:right.word}
  | fork
    ( pass )
    ( from coinflips@trial
      | c:=count()
      | values f"There were {c} flips" )
  | sort this'
```
=>
```mdtest-output
"There were 3 flips"
{flip:1,result:"heads",word:"one"}
{flip:2,result:"tails",word:"two"}
```

---

#### Expression Example

_Read from dynamically defined files and add a column_

```mdtest-command
echo '{a:1}{a:2}' > a.sup
echo '{b:3}{b:4}' > b.sup
echo '"a.sup" "b.sup"' | super -s -c "from f'{this}' | c:=coalesce(a,b)+1" -
```
=>
```mdtest-output
{a:1,c:2}
{a:2,c:3}
{b:3,c:4}
{b:4,c:5}
```

---
