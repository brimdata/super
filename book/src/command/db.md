## Synopsis

`super db` is a sub-command of [`super`](./super.md) to manage and query SuperDB data lakes.
You can import data from a variety of formats and it will automatically
be committed in [super-structured](../formats/_index.md)
format, providing full fidelity of the original format and the ability
to reconstruct the original data without loss of information.

SuperDB data lakes provide an easy-to-use substrate for data discovery, preparation,
and transformation as well as serving as a queryable and searchable store
for super-structured data both for online and archive use cases.

### Command

&emsp; **super db** &mdash; invoke SuperDB on a lakehouse

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

* `-h` display help
* `-hidden`  show hidden options

### Description

