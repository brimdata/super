# Installation

SuperDB, along with its new query language [SuperSQL](../super-sql/intro.md),
is downloadable software available as a single binary embodied in
the [super](../command/super.md) command.

You can [install a pre-built binary](#homebrew)
or [build from source code](#building-from-source).

To install the SuperDB Python client, see the
[Python library documentation](../dev/libraries/python.md).

## Homebrew

On macOS and Linux, you can use [Homebrew](https://brew.sh/) to install `super`:

```bash
brew install --cask brimdata/tap/super
```
Once installed, run a [quick test](hello-world.md).

## Building From Source

>[!TIP]
> If you don't have Go installed, download and install it from the
> [Go install page](https://golang.org/doc/install). Go 1.24 or later is
> required.

With Go installed, you can easily build `super` from source:

```bash
go install github.com/brimdata/super/cmd/super@main
```

This installs the `super` binary in your `$GOPATH/bin`.

## Try It

Once installed, run a [quick test](hello-world.md).
