# FROM

The `FROM` clause of a [SELECT](select.md) has the form
```
FROM <table-expr> [ , <table-expr> ... ]
```
where `<table-expr>` is a table expression having one of the forms:
```
<entity> [ ( <options> ) ] [ <as> ]
( <pipe-query> ) [ <as> ]
<named-query> [ <as> ]
```

`<entity>` is defined as in the pipe form of [from](../operators/from.md), namely one of
* a [text entity](../queries.md#text-entity) representing a file, URL, or   pool name,
* an [f-string](../expressions/f-strings.md) representing a file, URL, or pool name,
* a [glob](../queries.md#glob) matching files in the local file system or pool names in a database, or
* a [regular expression](../queries.md#regular-expression) matching pool names

`<options>` are the [entity options](../operators/from.md#options)
   as in pipe `from`.

`<pipe-query>` is any [query](../queries.md) inclusive of
[SQL pipe operators](intro.md###sql-pipe-operators).

`<named-query>` is the name of a [declared query](../declarations/queries.md).

All of the table expressions above may be bound to a table alias
with the option `<as>` clause of the form
```
[ AS ] <alias>
```
where the `AS` keyword is optional and `<alias>` has the form
```
<table> [ ( <column> [ , <column> ... ] ) ]
```
`<table>` and `<column>` are [identifiers](../queries.md#identifiers)
naming a table or a table and the columns of the indicated table
and an optional parenthesized list of columns positionally specifies the
column names of that table.

## Description

A `FROM` clause is a component of [SELECT](select.md) that
identifies the query's input data and creates a namespace for the
input comprised of table and column [references](intro.md#indentifier-resolution)
that may then in the various expressions appearing throughout the query.

The input data is indicated by one or more table expressions.
When there are multiple table expressions, the tables are combined
with relational joins into an single output table that is referenced
by the consistuent table and column names.


>[!NOTE]
> The SQL `FROM` clause is similar to the pipe form of the
> [from](../operators/from.md) operator but
> * uses [relational scoping](../intro.md#relational-scoping) instead of
>   [pipe scoping](../intro.md#pipe-scoping),
> * allows the binding of table aliases to relational data sources, and
> * can be combined with [JOIN](join.md) clauses to implement relational joins.

## File Examples

---

_Source structured data from a local file_

```mdtest-command
echo '{"greeting":"hello world!"}' > hello.json
super -s -c 'SELECT greeting FROM hello.json'
```
```mdtest-output
{greeting:"hello world!"}
```

---

_Source data from a local file, but in "line" format_
```mdtest-command
super -s -c 'SELECT this as line FROM hello.json (format line)'
```
```mdtest-output
{line:"{\"greeting\":\"hello world!\"}"}
```

## HTTP Example

---

_Source data from a URL_
```
super -s -c "SELECT name FROM https://raw.githubusercontent.com/brimdata/super/main/package.json"
```
```
{name:"super"}
```

---

## Database Examples

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
super db -f text -c '
  from :branches
  | values pool.name + "@" + branch.name
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
super db -db example -s -c 'SELECT * FROM coinflips'
```
```mdtest-output
{flip:2,result:"tails"}
{flip:1,result:"heads"}
```

---

_Source data from a specific branch of a pool_
```mdtest-command
super db -db example -s -c 'SELECT * FROM coinflips@trial'
```
```mdtest-output
{flip:3,result:"heads"}
{flip:2,result:"tails"}
{flip:1,result:"heads"}
```

---

_Count the number of values in the `main` branch of all pools_
```mdtest-command
super db -db example -f text -c 'SELECT count() FROM *'
```
```mdtest-output
5
```

---

_Join the data from multiple pools_

```mdtest-command
super db -db example -s -c '
  SELECT c.flip,c.result
  FROM coinflips c
  JOIN numbers n on c.flip=n.number
  ORDER BY flip
'
```
```mdtest-output
{flip:1,result:"heads"}
{flip:2,result:"tails"}
```

---

_Use `pass` to combine our join output with data from yet another source_
```mdtest-command
super db -db example -s -c "
  SELECT *
  FROM coinflips c
  JOIN numbers n
    ON c.flip=n.number
  | fork
    ( pass )
    ( SELECT f'There were {count()} flips' AS msg
      FROM coinflips@trial )
  | order by this
"
```
```mdtest-output
{msg:"There were 3 flips"}
{flip:1,result:"heads",number:1,word:"one"}
{flip:2,result:"tails",number:2,word:"two"}
```

---

### F-String Example

_Read from dynamically defined files and add a column_

```mdtest-command
echo '{a:1}{a:2}' > a.sup
echo '{b:3}{b:4}' > b.sup
echo '"a.sup" "b.sup"' | super -s -c "
SELECT *, coalesce(a,b)+1 AS c
FROM f'{this}'
" -
```
```mdtest-output
{a:1,c:2}
{a:2,c:3}
{b:3,c:4}
{b:4,c:5}
```
