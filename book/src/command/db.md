# super db

```
super [ options ] db [ options ] -c <query>
super [ options ] db <sub-command> ...
```

`super db` is a sub-command of [`super`](super.md) to manage and query SuperDB databases.

You can import data from a variety of formats and it will automatically
be committed in [super-structured](../formats/intro.md)
format, providing full fidelity of the original format and the ability
to reconstruct the original data without loss of information.

A SuperDB database offers an easy-to-use substrate for data discovery, preparation,
and transformation as well as serving as a queryable and searchable store
for super-structured data both for online and archive use cases.

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

By default, commands that display lake metadata (e.g., [`log`](db-log.md) or
[`ls`](db-ls.md)) use a text format.  However, the `-f` option can be used
to specify any supported [output format](super.md#supported-formats).

## Options

* `-configdir` configuration and credentials directory
* `-db database` location (env SUPER_DB)
* `-q` quiet mode (default "false")

## super db auth

```
super db auth login|logout|method|verify
```

Command-line options:
* [super db options](#options)

> **TODO: rename this command. it's really about connecting to a database.
> authenticating is something you do to connect.**
    login - log in to a database service and save credentials
    logout - remove saved credentials for a database service
    method - display authentication method supported by database service
    verify - verify authentication credentials

## super db branch
```
super db branch [options] [name]
```

Command-line options:
* [super db options](#options)

The `branch` command creates a branch with the name `name` that points
to the tip of the working branch or, if the `name` argument is not provided,
lists the existing branches of the selected pool.

For example, this branch command
```
super db branch -use logs@main staging
```
creates a new branch called "staging" in pool "logs", which points to
the same commit object as the "main" branch.  Once created, commits
to the "staging" branch will be added to the commit history without
affecting the "main" branch and each branch can be queried independently
at any time.

Supposing the `main` branch of `logs` was already the working branch,
then you could create the new branch called "staging" by simply saying
```
super db branch staging
```
Likewise, you can delete a branch with `-d`:
```
super db branch -d staging
```
and list the branches as follows:
```
super db branch
```

* [branch](db-branch.md)
* [compact](db-compact.md)
* [create](db-create.md)
* [delete](db-delete.md)
* [drop](db-drop.md)
* [init](db-init.md)
* [load](db-load.md)
* [log](db-log.md)
* [ls](db-ls.md)
* [manage](db-manage.md)
* [merge](db-merge.md)
* [query](db-query.md) **TODO: ref this doc**
* [rename](db-rename.md)
* [revert](db-revert.md)
* [serve](db-serve.md)
* [use](db-use.md)
* [vacate](db-vacate.md)
* [vacuum](db-vacuum.md)
* [vector](db-vector.md)

### Options

TODO



### Database Connection

> **TODO: document database location**

#### Commitish

> **TODO: document this somewhere maybe not here**

#### Sort Key
