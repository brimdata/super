---
title: Installation
---

Several options for installing `super` are available:
* [Homebrew](#homebrew) for Mac or Linux,
* [Binary download](#binary-download), or
* [Build from source](#building-from-source).

To install the SuperDB Python client, see the
[Python library documentation](libraries/python.md).

## Homebrew

On macOS and Linux, you can use [Homebrew](https://brew.sh/) to install `super`:

```bash
brew install brimdata/tap/super
```

Once installed, run a [quick test](#quick-tests).

## Binary Download

We offer pre-built binaries for macOS, Windows and Linux for both amd64/x86 and arm
architectures in the super [GitHub Release page](https://github.com/brimdata/super/releases).

Once the `super` binary is unpacked from a downloaded package, run a [quick test](#quick-tests).

## Building From Source

If you have Go installed, you can easily build `super` from source:

```bash
go install github.com/brimdata/super/cmd/super@latest
```

This installs the `super` binary in your `$GOPATH/bin`.

Once installed, run a [quick test](#quick-tests).

{{% tip "Note" %}}

If you don't have Go installed, download and install it from the
[Go install page](https://golang.org/doc/install). Go 1.23 or later is
required.

{{% /tip %}}

## Quick Tests

`super` is easy to test as it's a completely self-contained
command-line tool and requires no external dependencies.

### Test `super`

To test `super`, simply run this command in your shell:
```mdtest-command
echo '"hello, world"' | super -z -
```
which should produce
```mdtest-output
"hello, world"
```

### Test `super db`

To test `super db`, we'll make a SuperDB data lake in `./scratch`, load data, and query it
as follows:
```
export SUPER_DB_LAKE=./scratch
super db init
super db create Demo
echo '{s:"hello, world"}' | super db load -use Demo -
super db query "from Demo"
```
which should display
```
{s:"hello, world"}
```
Alternatively, you can run a lake service, load it with data using `super db load`,
and hit the API.

In one shell, run the server:
```
super db init -lake scratch
super db serve -lake scratch
```
And in another shell, run the client:
```
super db create Demo
super db use Demo
echo '{s:"hello, world"}' | super db load -
super db query "from Demo"
```
which should also display
```
{s:"hello, world"}
```
