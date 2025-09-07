# SuperSQL parser

This directory contains the SuperSQL parser implemented in PEG.

There is a single PEG input file that works with
[pigeon](https://github.com/mna/pigeon) to generate the Go parser.

## Build

To build the parser, just run make:

`make`

This will ensure the required libraries are installed and then produce the Go
parser (parser.go).

## Testing

The `super compile` command can be used for easily testing the output of
the parser.

## Development

During development, the easiest way to run the parser
is with this `make` command at the root of this repository:
```
make peg
```
This will ensure the PEG-generated Go parser is up to date with `parser.peg`

To update the parser and launch the `zc -repl`, your can run `make peg-run`.
