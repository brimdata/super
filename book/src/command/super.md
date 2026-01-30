# super

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

## Errors

Fatal errors like "file not found" or "file system full" are reported
as soon as they happen and cause the `super` process to exit.

On the other hand,
runtime errors resulting from the query itself
do not halt execution.  Instead, these error conditions produce
[first-class errors](../super-sql/types/error.md)
in the data output stream interleaved with any valid results.
Such errors are easily queried with the
[is_error](../super-sql/functions/errors/is_error.md) function.

This approach provides a robust technique for debugging complex queries,
where errors can be wrapped in one another providing stack-trace-like debugging
output alongside the output data.  This approach has emerged as a more powerful
alternative to the traditional technique of looking through logs for errors
or trying to debug a halted query with a vague error message.

For example, this query
```mdtest-command
echo '1 2 0 5' | super -s -c '10/this' -
```
produces
```mdtest-output
10
5
error("divide by zero")
2
```
and
```mdtest-command
echo '1 2 0 5' | super -c '10/this' - | super -s -c 'is_error(this)' -
```
produces just
```mdtest-output
error("divide by zero")
```

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
