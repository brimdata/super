# Command Line Interface (CLI)

`super` is the command-line tool for interacting with and managing SuperDB.
The command is organized as a hierarchy of sub-commands similar to
[`docker`](https://docs.docker.com/engine/reference/commandline/cli/)
or [`kubectl`](https://kubernetes.io/docs/reference/generated/kubectl/kubectl-commands).

The dependency-free command is [easy to install](../getting-started/install.md).

SuperDB does not have a [REPL like SQLite](https://sqlite.org/cli.html).
Instead, your shell is your REPL and the `super` command lets you:
* run SuperSQL queries [detached from](#running-a-query) a database,
* run SuperSQL queries [attached to](db.md#running-a-query) a database,
* [compile](compile.md) and inspect query plans,
* run a [SuperDB service](db.md#super-db-serve) endpoint,
* or access built-in [dev tooling](dev.md) when you want to dive deep.

The `super` command is invoked either by itself to run a query:
```
super [ -c <query> | -I <query-file> ] [ options ] [ <path> ... ]
```
or with a [sub-command](sub-commands.md):
```
super [ options ] <sub-command> ...
```

## Running a Query

When invoked at the top level without a sub-command (and either
a query or input paths are specified), `super` executes the
SuperDB query engine detached from the database storage layer.

The [input data](input.md) may be specified as command-line paths or
referenced within the query.

For built-in command help and a listing of all available options,
simply run `super` without any arguments.

### Options

When running a query detached from the database, the options include:

* [Global](options.md#global)
* [Query](options.md#query)
* [Input](options.md#input)
* [Output](options.md#output)

An optional [SuperSQL](../super-sql/intro.md)
query may be present via a `-c` or `-I` [option](options.md#query).

If no query is provided, the input paths are scanned
and output is produced in accordance with `-f` to specify a serialization format
and `-o` to specify an optional output (file or directory).

## Magic Mode

`super` also offers a unique, AI-enabled "magic mode" where it is able to
anticipate the query that would be most useful to execute next and will
output it alongside the results from your prior query. This feature can be
invoked using the `-m` flag.

## Debugging

If you are ever stumped about how the `super` compiler is parsing your query,
you can always run `super -C` to compile and display your query in canonical form
without running it.
This can be especially handy when you are learning the language and its
[shortcuts](../super-sql/operators/intro.md#shortcuts).

For example, this query
```mdtest-command
super -C -c 'has(foo)'
```
is an implied [where](../super-sql/operators/where.md) operator, which matches values
that have a field `foo`, i.e.,
```mdtest-output
where has(foo)
```
while this query
```mdtest-command
super -C -c 'a:=x+1'
```
is an implied [put](../super-sql/operators/put.md) operator, which creates a new field `a`
with the value `x+1`, i.e.,
```mdtest-output
put a:=x+1
```

You can also insert a [debug](../super-sql/operators/debug.md) operator anywhere in your
query, which lets you tap a complex query, filter the values, and trace the computation using
an arbitrary expression.  When running on the command-line, `super` displays debug
output on standard error.
