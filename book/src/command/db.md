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

By default, commands that display database metadata (e.g., [log](#super-db-log) or
[`ls`](db-ls.md)) use a text format.  However, the `-f` option can be used
to specify any supported [output format](super.md#supported-formats).

## Options

* `-configdir` configuration and credentials directory
* `-db database` location (env SUPER_DB)
* `-q` quiet mode (default "false")

## Sub-commands XXX
s
* [branch](#super-db-branch) xxx
* [compact](#super-db-compact) xxx
* [create](#super-db-create) create a new pool in a database
* [delete](#super-db-delete) delete data from a pool
* [drop](#super-db-drop) remove a pool from a database
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

## super db compact

```
super db compact id id [ id... ]
```

The `compact` command takes a list of one or more
data object IDs, writes the values
in those objects to a sequence of new, non-overlapping objects, and
creates a commit on HEAD replacing the old objects with the new ones.
    
## super db create

```
super db create [-orderby key[,key...][:asc|:desc]] <name>
```

**Options**
* `-orderby key` pool key with optional :asc or :desc suffix to organize data in pool (cannot be changed) (default "ts:desc")
* `-S size` target size of pool data objects, as '10MB' or '4GiB', etc. (default "500MiB")
* `-use pool` set created pool as the current pool (default "false")
* [super db options](#options)

The `create` command creates a new data pool with the given name,
which may be any valid UTF-8 string.

The `-orderby` option indicates the [sort key](#sort-key) that is used to sort
the data in the pool, which may be in ascending or descending order.

If a sort key is not specified, then it defaults to
the [special value `this`](../super-sql/intro.md#pipe-scoping).

A newly created pool is initialized with a branch called `main`.

> [!NOTE]
> Pools can be used without thinking about branches.  When referencing a pool without
> a branch, the tooling presumes the "main" branch as the default, and everything
> can be done on main without having to think about branching.

## super db delete

```
super db delete [options] <id> [<id>...]
super db delete [options] -where <filter>
```

**Options**
* `-message text` commit message
* `-meta value` application metadata
* `-use commitish` commit to use, i.e., pool, pool@branch, or pool@commit
* `-user user` user name for commit message
* `-where predicate` delete by any SuperSQL predicate
* [super db options](#options)

The `delete` command removes one or more data objects indicated by their ID from a pool.
This command
simply removes the data from the branch without actually deleting the
underlying data objects thereby allowing time travel to work in the face
of deletes.  Permanent deletion of underlying data objects is handled by the
separate [`vacuum`](#super-db-vacuum) command.

If the `-where` flag is specified, delete will remove all values for which the
provided filter expression is true.  The value provided to `-where` must be a
single filter expression, e.g.:

```
super db delete -where 'ts > 2022-10-05T17:20:00Z and ts < 2022-10-05T17:21:00Z'
```


## super db drop

```
super db drop [options] <name>|<id>
```

**Options**
* `-f` do not prompt for confirmation
* [super db options](#options)

The `drop` command deletes a pool and all of its constituent data.
As this is a DANGER ZONE command, you must confirm that you want to delete
the pool to proceed.  The `-f` option can be used to force the deletion
without confirmation.

### Database Connection

> **TODO: document database location**

#### Commitish

> **TODO: document this somewhere maybe not here**

#### Sort Key
