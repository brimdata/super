# Options

Each sub-command typically has a set of command-line options that
control its behavior but also inherits shared command-line options
based on the personality of the command, e.g., whether it runs
a [query](#query), [takes input](#input), [produces output](#output),
and so forth.

The shared options are documented here while each sub-command description
documents its command-specific options.

## Global

The global options are shared by all commands and include:
* `-h` display help
* `-help` display help
* `-hidden` show hidden options
* `-signals` catch `SIGINT`, `SIGTERM`, and `SIGPIPE` and exit gracefully
* `-version` print version and exit

The `-signals` option is off by default except for
[super db serve](db.md#super-db-serve), which always enables
the catching of `SIGINT` and `SIGTERM` (but not `SIGPIPE`).  When `-signals` is enabled,
the `super` command is interrupted but continues to a natural stopping point (e.g., flushing partial output files, removing spill files from temporary storage, etc.).
In the current implementation,
this can be unpredictably long so the default is to exit immediately,
potentially leaving behind corrupt outputs, etc.

## Query

The query options are available to commands that invoke the query runtime,
e.g., `super -c` and `super db -c`:

* `-aggmem` maximum memory used per aggregate function value in MiB, MB, etc
* `-c` [SuperSQL](../super-sql/intro.md) query to execute (may be used multiple times)
* `-e` stop upon input errors
* `-fusemem` maximum memory used by fuse in MiB, MB, etc
* `-I` source file containing query text (may be used multiple times)
* `-q` don't display warnings
* `-sortmem` maximum memory used by sort in MiB, MB, etc
* `-stats` display search stats on stderr

## Input

The input command-line options are available to commands that take input,
e.g., `super -c`, `super db load`:
* `-bsup.readmax` maximum Super Binary read buffer size in MiB, MB, etc.
* `-bsup.readsize` target Super Binary read buffer size in MiB, MB, etc.
* `-bsup.threads` number of Super Binary read threads
* `-bsup.validate` validate format when reading Super Binary
* `-csv.delim` CSV field delimiter
* `-e` stop upon input errors
* `-i` format of input data

## Output

The output command-line options are available to commands that produce output:
* `-B` allow Super Binary to be sent to a terminal output
* `-bsup.compress` compress Super Binary frames
* `-bsup.framethresh` minimum Super Binary frame size in uncompressed bytes (default "524288")
* `-color` enable/disable color formatting for -S and db text output
* `-f` format for output data
* `-J` shortcut for `-f json -pretty`, i.e., multi-line JSON
* `-j` shortcut for `-f json -pretty=0`, i.e., line-oriented JSON
* `-o` write data to output file
* `-persist` regular expression to persist type definitions across the stream
* `-pretty` tab size to pretty print JSON and Super JSON output
* `-S` shortcut for `-f sup -pretty`, i.e., multi-line SUP
* `-s` shortcut for `-f sup -pretty=0`, i.e., line-oriented SUP
* `-split split` output into one file per data type in this directory
* `-splitsize` if >0 and -split is set, split into files at least this big rather than by data type
* `-unbuffered` disable output buffering

An optional [SuperSQL](../super-sql/intro.md)
query is comprised of text specified by `-c` and source files
specified by `-I`.  Both `-c` and `-I` may appear multiple times and the
query text is concatenated in left-to-right order with intervening newlines.
Any error messages are properly collated to the included file
in which they occurred.

## Database

Database options available to the `super db` command and
to all `db` [sub-commands](db.md#sub-commands) include:
* `-configdir` configuration and credentials directory
* `-db database` location (defaults to environment variable SUPER_DB if not specified)
* `-q` quiet mode to disable displaying status messages for many `super db` subcommands

## Commit

Commit options are available to the `super db` commands that
create a commit in the database:
* `-user <text>` user name for commit message
* `-message <text>` commit message
* `-meta <value>` application metadata attached to commit
