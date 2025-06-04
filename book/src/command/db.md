## Synopsis

`super db` is a sub-command of [`super`](./super.md) to manage and query SuperDB databases.
You can import data from a variety of formats and it will automatically
be committed in [super-structured](../formats/_index.md)
format, providing full fidelity of the original format and the ability
to reconstruct the original data without loss of information.

A SuperDB database offers an easy-to-use substrate for data discovery, preparation,
and transformation as well as serving as a queryable and searchable store
for super-structured data both for online and archive use cases.

### Command

&emsp; **super db** &mdash; invoke SuperDB on a super-structured database

### Synopsis

```
super [ options ] db [ options ] -c <query>
super [ options ] db <sub-command> ...
```
### Sub-commands

* [compile](compile.md)
* [db](db.md)
* [dev](dev.md)

### Options

TODO

### Description

While `super db` is itself a sub-command of [`super`](super.md), it invokes
a large number of interrelated sub-commands, similar to the
[`docker`](https://docs.docker.com/engine/reference/commandline/cli/)
or [`kubectl`](https://kubernetes.io/docs/reference/generated/kubectl/kubectl-commands)
commands.

The following sections describe each of the available commands and highlight
some key options.  Built-in help shows the commands and their options:

* `super db -h` with no args displays a list of `super db` commands.
* `super db command -h`, where `command` is a sub-command, displays help
for that sub-command.
* `super db command sub-command -h` displays help for a sub-command of a
sub-command and so forth.

By default, commands that display lake metadata (e.g., [`log`](#log) or
[`ls`](#ls)) use the human-readable [lake metadata output](super.md#superdb-data-lake-metadata-output)
format.  However, the `-f` option can be used to specify any supported
[output format](super.md#output-formats).
