# super db

```
super [ options ] db [ options ] -c <query> | -I <query-file>
super [ options ] db <sub-command> ...
```

`super db` is a sub-command of [`super`](super.md) to manage and query SuperDB databases.

>[!NOTE]
> The database portion of SuperDB is early in development.  While previous versions
> have been deployed in production use at non-trvial scale, the current version
> is somewhat out of date with recent changes to the runtime.  This will be remedied
> in forthcoming releases.

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

## Concepts

### Database Connection

> **TODO: document database location**

### Commitish

> **TODO: document this somewhere maybe not here**

### Sort Key


## Query

When run without a sub-command and with one or more `-c` or `-I` arguments,
the specified query is on the database.

**Options**
* [super options](super.md#options)
* [super db options](#options)


The `query` command runs a [SuperSQL](../super-sql/intro.md) query with data from a lake as input.
A query typically begins with a [`from` operator](../super-sql/operators/from.md)
indicating the pool and branch to use as input.

The pool/branch names are specified with `from` in the query.

As with [`super`](super.md), the default output format is SUP for
terminals and BSUP otherwise, though this can be overridden with
`-f` to specify one of the various supported output formats.

If a pool name is provided to `from` without a branch name, then branch
"main" is assumed.

This example reads every record from the full key range of the `logs` pool
and sends the results to stdout.

```
super db query 'from logs'
```

We can narrow the span of the query by specifying a filter on the database
[sort key](db.md#sort-key):
```
super db query 'from logs | ts >= 2018-03-24T17:36:30.090766Z and ts <= 2018-03-24T17:36:30.090758Z'
```
Filters on sort keys are efficiently implemented as the data is laid out
according to the sort key and seek indexes keyed by the sort key
are computed for each data object.

When querying data to the [BSUP](../formats/bsup.md) output format,
output from a pool can be easily piped to other commands like `super`, e.g.,
```
super db query -f bsup 'from logs' | super -f table -c 'count() by field' -
```
Of course, it's even more efficient to run the query inside of the pool traversal
like this:
```
super db query -f table 'from logs | count() by field'
```
By default, the `query` command scans pool data in sort-key order though
the query optimizer may, in general, reorder the scan to optimize searches,
aggregations, and joins.
An order hint can be supplied to the `query` command to indicate to
the optimizer the desired processing order, but in general,
the [sort](../super-sql/operators/sort.md) operator
should be used to guarantee any particular sort order.

Arbitrarily complex queries can be executed over the lake in this fashion
and the planner can utilize cloud resources to parallelize and scale the
query over many parallel workers that simultaneously access the lake data in
shared cloud storage (while also accessing locally- or cluster-cached copies of data).

#### Meta-queries

Commit history, metadata about data objects, database and pool configuration,
etc. can all be queried and
returned as super-structured data, which in turn, can be fed into analytics.
This allows a very powerful approach to introspecting the structure of a
lake making it easy to measure, tune, and adjust lake parameters to
optimize layout for performance.

These structures are introspected using meta-queries that simply
specify a metadata source using an extended syntax in the `from` operator.
There are three types of meta-queries:
* `from :<meta>` - lake level
* `from pool:<meta>` - pool level
* `from pool[@<branch>]<:meta>` - branch level

`<meta>` is the name of the metadata being queried. The available metadata
sources vary based on level.

For example, a list of pools with configuration data can be obtained
in the SUP format as follows:
```
super db query -S "from :pools"
```
This meta-query produces a list of branches in a pool called `logs`:
```
super db query -S "from logs:branches"
```
You can filter the results just like any query,
e.g., to look for particular branch:
```
super db query -S "from logs:branches | branch.name=='main'"
```

This meta-query produces a list of the data objects in the `live` branch
of pool `logs`:
```
super db query -S "from logs@live:objects"
```

You can also pretty-print in human-readable form most of the metadata records
using the "lake" format, e.g.,
```
super db query -f lake "from logs@live:objects"
```

The `main` branch is queried by default if an explicit branch is not specified,
e.g.,

```
super db query -f lake "from logs:objects"
```

## Sub-commands

* [auth](#super-db-auth) authentication and authorization commands
* [branch](#super-db-branch) create a new branch in a pool
* [compact](#super-db-compact) compact data objects on a pool branch
* [create](#super-db-create) create a new pool in a database
* [delete](#super-db-delete) delete data from a pool
* [drop](#super-db-drop) remove a pool from a database
* [init](#super-db-init) create and initialize a new database
* [load](#super-db-load) load data into database
* [log](#super-db-log) display the commit log
* [ls](#super-db-ls) list the pools in a database
* [manage](#super-db-manage) run regular maintenance on a database
* [merge](#super-db-merge) merged data from one branch to another
* [query](db-query.md) **TODO: ref this doc**
* [rename](#super-db-rename) rename a database pool
* [revert](#super-db-revert) revert reverses an old commit
* [serve](#super-db-serve)  run a SuperDB service endpoint
* [use](#super-db-use) set working branch for `db` commands
* [vacate](#super-db-vacate) compact a pool's commit history by squashing old commit objects
* [vacuum](#super-db-vacuum) vacuum deleted storage in database

### super db auth

```
super db auth login|logout|method|verify
```

**Options**
* [super db options](#options)

> **TODO: rename this command. it's really about connecting to a database.
> authenticating is something you do to connect.**
    login - log in to a database service and save credentials
    logout - remove saved credentials for a database service
    method - display authentication method supported by database service
    verify - verify authentication credentials

### super db branch
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

### super db compact

```
super db compact id id [ id... ]
```

The `compact` command takes a list of one or more
data object IDs, writes the values
in those objects to a sequence of new, non-overlapping objects, and
creates a commit on HEAD replacing the old objects with the new ones.
    
### super db create

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

### super db delete

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

### super db drop

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

### super db init

```
super db init [path]
```

**Options**
* [super db options](#options)

A new database is created and initialized with the `init` command.
The `path` argument is a
[storage path](../database/intro.md#storage-layer)
and is optional.  If not present, the path
is [determined automatically](#database-connection).

If the database already exists, `init` reports an error and does nothing.

Otherwise, the `init` command writes the initial cloud objects to the
storage path to create a new, empty database at the specified path.

### super db load

```
super db load [options] input [input ...]
```

**Options**
* [super db options](#options)

The `load` command commits new data to a branch of a pool.

Run `super db load -h` for a list of command-line options.

Note that there is no need to define a schema or insert data into
a "table" as all super-structured data is _self describing_ and can be queried in a
schema-agnostic fashion.  Data of any _shape_ can be stored in any pool
and arbitrary data _shapes_ can coexist side by side.

As with [`super`](super.md),
the [input arguments](super.md#options) can be in
any [supported format](super.md#supported-formats) and
the input format is auto-detected if `-i` is not provided.  Likewise,
the inputs may be URLs, in which case, the `load` command streams
the data from a Web server or [S3](../dev/integrations/s3.md)
and into the database.

When data is loaded, it is broken up into objects of a target size determined
by the pool's `threshold` parameter (which defaults to 500MiB but can be configured
when the pool is created).  Each object is sorted by the [sort key](db.md#sort-key) but
a sequence of objects is not guaranteed to be globally sorted.  When lots
of small or unsorted commits occur, data can be fragmented.  The performance
impact of fragmentation can be eliminated by regularly [compacting](db-manage.md)
pools.

For example, this command
```
super db load sample1.json sample2.bsup sample3.sup
```
loads files of varying formats in a single commit to the working branch.

An alternative branch may be specified with a branch reference with the
`-use` option, i.e., `<pool>@<branch>`.  Supposing a branch
called `live` existed, data can be committed into this branch as follows:
```
super db load -use logs@live sample.bsup
```
Or, as mentioned above, you can set the default branch for the load command
via [`use`](db-use.md):
```
super db use logs@live
super db load sample.bsup
```
During a `load` operation, a commit is broken out into units called _data objects_
where a target object size is configured into the pool,
typically 100MB-1GB.  The records within each object are sorted by the sort key.
A data object is presumed by the implementation
to fit into the memory of an intake worker node
so that such a sort can be trivially accomplished.

Data added to a pool can arrive in any order with respect to its sort key.
While each object is sorted before it is written,
the collection of objects is generally not sorted.

Each load operation creates a single [commit](../database/intro.md#commit-objects),
which includes:
* an author and message string,
* a timestamp computed by the server, and
* an optional metadata field of any type expressed as a Super (SUP) value.
This data has the type signature:
```
{
    author: string,
    date: time,
    message: string,
    meta: <any>
}
```
where `<any>` is the type of any optionally attached metadata .
For example, this command sets the `author` and `message` fields:
```
super db load -user user@example.com -message "new version of prod dataset" ...
```
If these fields are not specified, then the system will fill them in
with the user obtained from the session and a message that is descriptive
of the action.

The `date` field here is used by the database for
[time travel](../database/intro.md#time-travel)
through the branch and pool history, allowing you to see the state of
branches at any time in their commit history.

Arbitrary metadata expressed as any [SUP value](../formats/sup.md)
may be attached to a commit via the `-meta` flag.  This allows an application
or user to transactionally commit metadata alongside committed data for any
purpose.  This approach allows external applications to implement arbitrary
data provenance and audit capabilities by embedding custom metadata in the
commit history.

Since commit objects are stored as super-structured data, the metadata can easily be
queried by running the `log -f bsup` to retrieve the log in BSUP format,
for example, and using [`super`](super.md) to pull the metadata out
as in:
```
super db log -f bsup | super -c 'has(meta) | values {id,meta}' -
```

### super db log

```
super db log [options] [commitish]
```

**Options**
* [super db options](#options)

The `log` command, like `git log`, displays a history of the
[commits](../database/intro.md#commit-objects)
starting from any commit, expressed as a [commitish](db.md#commitish).  If no argument is
given, the tip of the working branch is used.

Run `super db log -h` for a list of command-line options.

To understand the log contents, the `load` operation is actually
decomposed into two steps under the covers:
an "add" step stores one or more
new immutable data objects in the lake and a "commit" step
materializes the objects into a branch with an ACID transaction.
This updates the branch pointer to point at a new commit object
referencing the data objects where the new commit object's parent
points at the branch's previous commit object, thus forming a path
through the object tree.

The `log` command prints the commit ID of each commit object in that path
from the current pointer back through history to the first commit object.

A commit object includes
an optional author and message, along with a required timestamp,
that is stored in the commit journal for reference.  These values may
be specified as options to the [`load`](db-load.md) command, and are also available in the
database [API](../database/api.md) for automation.

>[!NOTE]
> The branchlog meta-query source is not yet implemented.

### super db ls

```
super db ls [options] [pool]
```

**Options**
* [super db options](#options)

The `ls` command lists pools in a database or branches in a pool.

By default, all pools in the database are listed along with each pool's unique ID
and [sort key](db.md#sort-key).

If a pool name or pool ID is given, then the pool's branches are listed along
with the ID of their commit object, which points at the tip of each branch.

### super db manage

```
super db manage [options]
```

**Options**
* `-config path` path of manage YAML config file
* `-interval duration` interval between updates (applicable only with -monitor)
* `-log.devmode` development mode
* `-log.filemode`
* `-log.level level` logging level (default "info")
* `-log.path path` path to send logs (values: stderr, stdout, path in file system) (default "stderr")
* `-monitor` continuously monitor the database for updates
* `-pool pool` pool to manage (all if unset, can be specified multiple times)
* `-vectors` create vectors for objects
* [super db options](#options)

The `manage` command performs maintenance tasks on a database.

Currently the only supported task is _compaction_, which reduces fragmentation
by reading data objects in a pool and writing their contents back to large,
non-overlapping objects.

If the `-monitor` option is specified and the database is
[configured](db.md#database-connection)
via network connection, `super db manage` will run continuously and perform updates
as needed.  By default a check is performed once per minute to determine if
updates are necessary.  The `-interval` option may be used to specify an
alternate check frequency as a [duration](../super-sql/types/time.md).

If `-monitor` is not specified, a single maintenance pass is performed on the
database.

By default, maintenance tasks are performed on all pools in the database.  The
`-pool` option may be specified one or more times to limit maintenance tasks
to a subset of pools listed by name.

The output from `manage` provides a per-pool summary of the maintenance
performed, including a count of `objects_compacted`.

As an alternative to running `manage` as a separate command, the `-manage`
option is also available on the [`serve`](#super-db-serve) command to have maintenance
tasks run at the specified interval by the service process.

### super db merge

```
super db merge -use logs@updates <branch>
```

**Options**
* `-f` force merge of main into a target (default "false")
* `-message text` commit message
* `-meta value` application metadata
* `-use commitish` commit to use, i.e., pool, pool@branch, or pool@commit
* `-user name` user name for commit message (default "mccanne@bridge.local")
* [super db options](#options)

Data is merged from one branch into another with the `merge` command, e.g.,
```
super db merge -use logs@updates main
```
where the `updates` branch is being merged into the `main` branch
within the `logs` pool.

A merge operation finds a common ancestor in the commit history then
computes the set of changes needed for the target branch to reflect the
data additions and deletions in the source branch.
While the merge operation is performed, data can still be written concurrently
to both branches and queries performed and everything remains transactionally
consistent.  Newly written data remains in the
branch while all of the data present at merge initiation is merged into the
parent.

This Git-like behavior for a data lake provides a clean solution to
the live ingest problem.
For example, data can be continuously ingested into a branch of `main` called `live`
and orchestration logic can periodically merge updates from branch `live` to
branch `main`, possibly [compacting](db-manage.md) data after the merge
according to configured policies and logic.

### super db rename

```
super db rename <existing> <new-name>
```

**Options**
* [super db options](#options)

The `rename` command assigns a new name `<new-name>` to an existing
pool `<existing>`, which may be referenced by its ID or its previous name.

### super db revert

```
super db revert commitish
```

**Options**
* `-message text` commit message
* `-meta value` application metadata
* `-use commitish` commit to use, i.e., pool, pool@branch, or pool@commit
* `-user name` user name for commit message (default "mccanne@bridge.local")
* [super db options](#options)

The `revert` command reverses the actions in a commit by applying the
inverse steps in a new commit to the tip of the indicated branch.  Any
data loaded in a reverted commit remains in the database but no longer
appears in the branch. The new commit may recursively be reverted by an
additional revert operation.

### super db use

```
super db use [ <commitish> ]
```
**Options**
* [super db options](#options)

The `use` command sets the working branch to the indicated commitish.
When run with no argument, it displays the working branch and
[database connection](db.md#database-connection).

For example,
```
super db use logs
```
provides a "pool-only" commitish that sets the working branch to `logs@main`.

If a `@branch` or commit ID are given without a pool prefix, then the pool of
the commitish previously in use is presumed.  For example, if you are on
`logs@main` then run this command:
```
super db use @test
```
then the working branch is set to `logs@test`.

To specify a branch in another pool, simply prepend
the pool name to the desired branch:
```
super db use otherpool@otherbranch
```
This command stores the working branch in `$HOME/.super_head`.

### super db serve

```
super db serve [options]
```

* `-auth.audience` [Auth0](https://auth0.com/) audience for API clients (will be publicly accessible)
* `-auth.clientid` [Auth0](https://auth0.com/) client ID for API clients (will be publicly accessible)
* `-auth.domain` [Auth0](https://auth0.com/) domain (as a URL) for API clients (will be publicly accessible)
* `-auth.enabled` enable authentication checks
* `-auth.jwkspath` path to JSON Web Key Set file
* `-cors.origin` CORS allowed origin (may be repeated)
* `-defaultfmt` default response format (default "sup")
* `-l [addr]:port` to listen on (default ":9867")
* `-log.devmode` development mode (if enabled dpanic level logs will cause a panic)
* `-log.filemod` logger file write mode (values: append, truncate, rotate)
* `-log.level` logging level
* `-log.path` path to send logs (values: stderr, stdout, path in file system)
* `-manage duration` when positive, run lake maintenance tasks at this interval
* `-rootcontentfile` file to serve for GET /
* [super db options](#options)

**TODO: get rid of personality metaphor?**

The `serve` command implements the
[server personality](../database/intro.md#command-personalities) to service requests
from instances of the client personality.
It listens for [API](../database/api.md) requests on the interface and port
specified by the `-l` option, executes the requests, and returns results.

The `-log.level` option controls log verbosity.  Available levels, ordered
from most to least verbose, are `debug`, `info` (the default), `warn`,
`error`, `dpanic`, `panic`, and `fatal`.  If the volume of logging output at
the default `info` level seems too excessive for production use, `warn` level
is recommended.

The `-manage` option enables the running of the same maintenance tasks
normally performed via the separate [`manage`](db-manage.md) command.

### super db vacate

```
super db vacate [ options ] commit
```
**Options**
* `-message text` commit message
* `-meta value` application metadata
* `-use commitish` commit to use, i.e., pool, pool@branch, or pool@commit
* `-user user` user name for commit message
* [super db options](#options)

The `vacate` command compacts the commit history by squashing all of the
commit objects in the history up to the indicated commit and removing
the old commits. No other commit objects in the pool may point at any
of the squashed commits. In particular, no branch may point to any
commit that would be deleted.

The branch history may contain pointers to old commit objects, but any
attempt to access them will fail as the underlying commit history will
be no longer available.

**DANGER ZONE.** There is no prompting or second chances here so use
carefully. Once the pool's commit history has been squashed and old
commits are deleted, they cannot be recovered.

### super db vacuum

```
super db vacuum [ options ]
```

**Options**
* `-dryrun` run vacuum without deleting anything
* `-f` do not prompt for confirmation
* `-use` specify commit to use, i.e., pool, pool@branch, or pool@commit
* [super db options](#options)

The `vacuum` command permanently removes underlying data objects that have
previously been subject to a [`delete`](db-delete.md) operation.

**DANGER ZONE.** You must confirm that you want to remove
the objects to proceed.  The `-f` option can be used to force removal
without confirmation.  The `-dryrun` option may also be used to see a summary
of how many objects would be removed by a `vacuum` but without removing them.
