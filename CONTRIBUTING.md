# Contributing

Thank you for contributing to SuperDB!

Per common practice, please engage with an
[issue](https://github.com/brimdata/super/issues) before sending a pull request,
either by opening a new one or by commenting on an existing one that covers your
intended change.  When doing so, please describe your intended approach and wait for
feedback from a maintainer before investing time writing code.  This helps avoid situations where
effort goes into a direction the team can't accept.  If you think your ideas might
benefit from some refinement via Q&A, come talk to us on
[Slack](https://www.brimdata.io/join-slack/) as well.

Regardless of what tools you use to write code, you as the contributor are responsible
for the correctness, design fit, and quality of what you submit.  A reviewable PR is
one where a human can explain *why* the design choices were made, not just *what*
changed.  PRs that show no evidence of prior discussion or human judgment applied to
the approach may be closed without review.

SuperDB is early in its life cycle and will be expanding quickly.  Please star and/or
watch the repo so you can follow and track our progress.

In particular, we will be adding many more operators and aggregate functions.
If you want a fun, small project to help out, pick some functionality that is missing and
add an operator in [runtime/sam/op](runtime/sam/op) or an aggregate function
in [runtime/sam/expr/agg](runtime/sam/expr/agg).

## Development

Compiling `super` requires Go 1.26 or later and uses [Go modules](https://github.com/golang/go/wiki/Modules).
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
