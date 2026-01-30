# Installation

SuperDB is downloadable software available as a single binary embodied
in the [super](../command/super.md) command.  This software includes support
for [SuperSQL](../super-sql/intro.md),
SuperDB's new query language for super-structured data.

Several options for installation are available:
* download and install pre-built binaries via links on the
  [GitHub Releases page](https://github.com/brimdata/super/releases/tag/v0.1.0),
* automatically install a pre-built binary for a Mac or Linux environment
  with [HomeBrew](#homebrew), or
* [build from source code](#building-from-source).

To install the SuperDB Python client, see the
[Python library documentation](../dev/libraries/python.md).

## Homebrew

On macOS and Linux, you can use [Homebrew](https://brew.sh/) to install `super`:

```bash
brew install --cask brimdata/tap/super
```

## Building From Source

With Go installed, you can easily build `super` from source:

```bash
go install github.com/brimdata/super/cmd/super@main
```

This installs the `super` binary in your `$GOPATH/bin`.

>[!TIP]
> If you don't have Go installed, download and install it from the
> [Go install page](https://golang.org/doc/install). Go 1.24 or later is
> required.

## Try It

Once installed, run a [quick test](hello-world.md).
