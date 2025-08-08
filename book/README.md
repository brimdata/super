# SuperDB docs

This directory contains the user documentation for all of the SuperDB system.

## Working on docs

You'll need `mdbook`.  Install it with brew
```
brew install mdbook
```

The easiest way to work on docs is to run an mdbook service in this directory
and point your browser at its embedded web server, e.g.,
```
cd book
make
mdbook serve
```
Then connect to localhost:3000.

To add or remove sections of the book edit `src/SUMMARY.md`.

When editing `SUMMARY.md` it can be useful to kill the mdbook service
and build the book manually like this:
```
mdbook build
```
This way the service doesn't do things like recreating a file that you have
removed when you are trying to rearrange things.

After editing any JavaScript or Go files run `make` in this directory.
This will update the Wasm file so playground examples use a version of
SuperDB built from Go source files in the local repository.

## mdtest

TBD
