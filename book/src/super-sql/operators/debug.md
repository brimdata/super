# debug

[✅](../intro.md#data-order)&ensp; tap a pipeline to emit debugging values

## Synopsis

```
debug [ <expr> ]
```

## Description

The `debug` operator passes its input unmodified to its output and for
each value in the input, evalutes the optional
[expression](../expressions/intro.md) `<expr>`,
and transmits each result to a _debugging output_.
If `<expr>` is omitted, then each input value is passed unmodified to the
debugging output.

When running a query with the [super](../../command/super.md) command,
the debugging output is printed to standard error.
In this case, debugging output is displayed in the [SUP](../../formats/sup.md) format
independent of the `-f` flag.

When running a query in a database service, the debugging output is transmitted
to the client on the output channel named "debug".  If the client in this case
is the [super db](../../command/super.md) command,
then the debugging output is printed to standard error.

>[!NOTE]
> A future version of SuperDB Desktop will support viewing debug output
> in a panel in the application.

## Examples

---

_Hello, world_
```mdtest-spq
# spq
values "hello, world" | debug {debug:this} | where false
# input

# expected output
{debug:"hello, world"}
```
