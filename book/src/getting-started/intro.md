# Getting Started

It's super easy to get going with SuperDB.

Short on time?  Just [browse the TL;DR](tldr.md).

Otherwise, try out the [embedded playground](playground.md) examples
throughout the documentation, or

* [install super](install.md), and
* [try it out](hello-world.md).

The [`super` command](../command/super.md) is a single binary
arranged into a hierarchy of sub-commands.

SuperDB's disaggregation of compute and storage is reflected into the
design of its command hierarchy: running the top-level `super` command
runs the compute engine only on inputs like files and URLs, while
the [`db`](../command/db.md) subcommands of `super` operate upon
a [persistent database](../database/intro.md).

To get online help, run the `super` command or any sub-command with `-h`,
e.g.,
```
super -h
```
displays help for the top-level command, while
```
super db load -h
```
displays help for loading data into a SuperDB database, and so forth.

>[!NOTE]
> While `super` and its accompanying data formats are production quality for
> many use cases, the project's [persistent database](../command/db.md)
> is a bit earlier in development.
