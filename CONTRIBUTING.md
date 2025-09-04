# Contributing

Thank you for contributing to `zed`!

Per common practice, please [open an issue](https://github.com/brimdata/super/issues)
before sending a pull request.  If you think your ideas might benefit from some
refinement via Q&A, come talk to us on [Slack](https://www.brimdata.io/join-slack/) as well.

`zed` is early in its life cycle and will be expanding quickly.  Please star and/or
watch the repo so you can follow and track our progress.

In particular, we will be adding many more operators and aggregate functions.
If you want a fun, small project to help out, pick some functionality that is missing and
add an operator in [runtime/sam/op](runtime/sam/op) or an aggregate function
in [runtime/sam/expr/agg](runtime/sam/expr/agg).

## Development

`zed` requires Go 1.24 or later, and uses [Go modules](https://github.com/golang/go/wiki/Modules).
Compilation for 32-bit target environments is not currently supported
(see [super/4044](https://github.com/brimdata/super/issues/4044)).
Dependencies are specified in the [`go.mod` file](./go.mod) and fetched
automatically by commands like `go build` and `go test`.  No explicit
fetch commands are necessary.  However, you must set the environment
variable `GO111MODULE=on` if your repo is at
`$GOPATH/src/github.com/brimdata/super`.

When `go.mod` or its companion `go.sum` are modified during development, run
`go mod tidy` and then commit the changes to both files.

To use a local checkout of a dependency, use `go mod edit`:
```
go mod edit -replace=github.com/org/repo=../repo
```

### Testing

Before any PRs are merged to main, all tests must pass.

Unit tests require Node.js.  To run them, execute:
```
make test-unit
```

System tests require Python 3.3 or better.  To run them, execute:
```
make test-system
```
