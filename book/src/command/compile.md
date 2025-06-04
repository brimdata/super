### Command

&emsp; **compile** &mdash;  compile a SuperSQL query for inspection and debugging

### Synopsis

```
super compile [ options ] query
```

### Options

* `-C` display DAG or AST as query text (default "false")
* `-dag` display output as DAG (implied by -O or -P) (default "false")
* `-files` compile query as if command-line input files are present) (default "false")
* `-I` source file containing query text (may be repeated)
* `-O` display optimized DAG (default "false")
* `-P` display parallelized DAG (default "0")

Additional options of the [super top-level command](super.md#options)

### Description

This command parses a [SuperSQL](../super-sql/intro.md) query
and emits the resulting abstract syntax tree (AST) or
runtime directed acyclic graph (DAG) in the output
format desired. Use `-dag` to specify the DAG form; otherwise, the
AST form is assumed.

The `-C` option causes the output to be shown as query language
source instead of the AST.  This is particularly helpful to
see how SQP queries in their abbreviated form are translated
into the exanded, pedantic form  of piped SQL.  The DAG can
also be formatted as query-style text but the resulting text is
informational only and does not conform to any query syntax.  When
`-C` is specified, the result is sent to stdout and the `-f` and
`-o` options have no effect.

This command is often used for dev and test but is also useful to
advanced users for understanding how SuperSQL syntax is parsed
into an AST or compiled into a runtime DAG.
