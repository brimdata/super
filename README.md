# `zq` [![CI][ci-img]][ci] [![GoDoc][doc-img]][doc]

`zq` is a command-line tool that's useful for searching and analyzing logs;
particularly [Zeek](https://www.zeek.org) logs.  If you are familiar with
[`zeek-cut`](https://github.com/zeek/zeek-aux/tree/master/zeek-cut),
you can think of `zq` as `zeek-cut` on steroids.  (If you missed
[the name change](https://blog.zeek.org/2018/10/renaming-bro-project_11.html),
Zeek was formerly known as "Bro".)

`zq` is comprised of
* an [execution engine](proc) for log pattern search and analytics,
* a [query language](pkg/zql/README.md) that compiles into a program that runs on
the execution engine, and
* an open specification for structured logs, called [ZNG](pkg/zng/docs/README.md).<br>
(**Note**: The ZNG format is in Alpha and subject to change.)

`zq` takes Zeek/ZNG logs as input and filters, transforms, and performs
analytics using the
[zq query language](pkg/zql/README.md),
producing a log stream as its output.

## Install

We don't yet distribute pre-built binaries, so to install `zq`, you must
clone the repo and compile the source.

If you don't have Go installed,
download it from the [Go downloads page](https://golang.org/dl/) and install it.
If you're new to Go, remember to set GOPATH.  A common convention is to create ~/go
and point GOPATH at $HOME/go.

To install the binaries in `$GOPATH/bin`, grab this repo and
execute a good old-fashioned `make install`:

```
git clone https://github.com/mccanne/zq
cd zq
make install
```
## Usage

For `zq` command usage, see the built-in help by running
```
zq help
```
`zq` program syntax and semantics are documented in the
[query language README](pkg/zql/README.md)

### Examples

Here are a few examples based on a very simple "conn" log from Zeek [(conn.log)](conn.log),
located in this directory.  See the
[zq-sample-data repo](https://github.com/mccanne/zq-sample-data ) for more
test data, which is used in the examples in the
[query language documentation](https://github.com/mccanne/zq/blob/master/pkg/zql/README.md).

To cut the columns of a Zeek "conn" log like `zeek-cut` does, run:
```
zq "* | cut ts,id.orig_h,id.orig_p" conn.log
```
The "`*`" tells `zq` to match every line, which is sent to the `cut` processor
using the UNIX-like pipe syntax.

When looking over everything like this, you can  omit the search pattern
as a shorthand and simply type:
```
zq "cut ts,id.orig_h,id.orig_p" conn.log
```

The default output is a ZNG file.  If you want just the tab-separated lines
like `zeek-cut`, you can specify text output:
```
zq -f text "cut ts,id.orig_h,id.orig_p" conn.log
```
If you want the old-style Zeek [ASCII TSV](https://docs.zeek.org/en/stable/examples/logs/)
log format, run the command with the `-f` flag specifying `zeek` for the output
format:
```
zq -f zeek "cut ts,id.orig_h,id.orig_p" conn.log
```
You can use an aggregate function to summarize data over one or
more fields, e.g., summing field values, counting, or computing an average.
```
zq "sum(orig_bytes)" conn.log
zq "orig_bytes > 10000 | count()" conn.log
zq "avg(orig_bytes)" conn.log
```

The [ZNG specification](pkg/zng/docs/spec.md) describes the significance of the
`_path` field.  By leveraging this, diverse Zeek logs can be combined into a single
file.
```
zq *.log > all.zng
```

### Comparisons

Revisiting the `cut` example shown above:

```
zq -f text "cut ts,id.orig_h,id.orig_p" conn.log
```

This is functionally equivalent to the `zeek-cut` command-line:

```
zeek-cut ts id.orig_h id.orig_p < conn.log
```

If your Zeek events are stored as JSON and you are accustomed to querying with `jq`,
the equivalent would be:

```
jq -c '. | { ts, "id.orig_h", "id.orig_p" }' conn.ndjson
```

Comparisons of other simple operations and their relative performance are described
at the [performance](performance/README.md) page.

## Development

`zq` is a [Go module](https://github.com/golang/go/wiki/Modules), so
dependencies are specified in the [`go.mod` file](/go.mod) and managed
automatically by commands like `go build` and `go test`.  No explicit
fetch commands are necessary.  However, you must set the environment
variable `GO111MODULE=on` if your repo is at
`$GOPATH/src/github.com/mccanne/zq`.

`zq` currently requires Go 1.13 or later, so make sure your install is up to date.

When `go.mod` or its companion `go.sum` are modified during development, run
`go mod tidy` and then commit the changes to both files.

To use a local checkout of a dependency, use `go mod edit`:
```
go mod edit -replace=github.com/org/repo=../repo
```

Note that local checkouts must have a `go.mod` file, so it may be
necessary to create a temporary one:
```
echo 'module github.com/org/repo' > ../repo/go.mod
```

### Testing

Before any PRs are merged to master, all tests must pass.

To run unit tests in your local repo, execute
```
make test-unit
```

And to run system tests, execute
```
make test-system
```


## Contributing

`zq` is developed on GitHhub by its community. We welcome contributions.

Feel free to
[post an issue](https://github.com/mccanne/zq/issues),
fork the repo, or send us a pull request.

`zq` is early in its life cycle and will be expanding quickly.  Please star and/or
watch the repo so you can follow and track our progress.

In particular, we will be adding many more processors and aggregate functions.
If you want a fun small project to help out, pick some functionality that is missing and
add a processor in
[zq/proc](proc)
or an aggregate function in
[zq/reducer](reducer).


[doc-img]: https://godoc.org/github.com/mccanne/zq?status.svg
[doc]: https://godoc.org/github.com/mccanne/zq
[ci-img]: https://circleci.com/gh/mccanne/zq.svg?style=svg
[ci]: https://circleci.com/gh/mccanne/zq
