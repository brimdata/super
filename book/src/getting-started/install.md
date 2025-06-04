## Installation

> **TODO: upon release, update this first paragraph.**

Because SuperDB is still under construction, GA releases are not yet available.
However, you can install a build of the [`super`](https://superdb.org/docs/commands/super)
command-line tool based on code that's under active development to start
tinkering.

Multiple options for installing `super` are available:
* [Homebrew](#homebrew) for Mac or Linux,
* [Build from source](#building-from-source).

To install the SuperDB Python client, see the
[Python library documentation](../dev/libraries/python.md).

### Homebrew

On macOS and Linux, you can use [Homebrew](https://brew.sh/) to install `super`:

```bash
brew install brimdata/tap/super
```
Once installed, run a [quick test](hello-world.md).

### Building From Source

> If you don't have Go installed, download and install it from the
> [Go install page](https://golang.org/doc/install). Go 1.24 or later is
> required.

With Go installed, you can easily build `super` from source:

```bash
go install github.com/brimdata/super/cmd/super@main
```

This installs the `super` binary in your `$GOPATH/bin`.

### Try It

Once installed, run a [quick test](hello-world.md).
