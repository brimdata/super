# Database

> **TODO: update and simplify.  add note about readiness**

## Data Pools

## Commitish

>[!NOTE]
> While `super` and its accompanying formats
> are production quality, SuperDB's persistent database is still fairly early in development
> and alpha quality.
> That said, SuperDB databases can be utilized quite effectively at small scale,
> or at larger scales when scripted automation
> is deployed to manage the lake's data layout via the database
> [API](../database/api.md).
>
> Enhanced scalability with self-tuning configuration is under development.

## Design Philosophy

XXX this section pasted in...fix

The design philosophy for SuperDB is based on composable building blocks
built from self-describing data structures.  Everything in a SuperDB data lake
is built from super-structured data and each system component can be run and tested in isolation.

Since super-structured data is self-describing, this approach makes stream composition
very easy.  Data from a query can trivially be piped to a local
instance of `super` by feeding the resulting output stream to stdin of `super`, for example,
```
super db query "from pool | ...remote query..." | super -c "...local query..." -
```
There is no need to configure the SuperDB entities with schema information
like [protobuf configs](https://developers.google.com/protocol-buffers/docs/proto3)
or connections to
[schema registries](https://docs.confluent.io/platform/current/schema-registry/index.html).

A SuperDB data lake is completely self-contained, requiring no auxiliary databases
(like the [Hive metastore](https://hive.apache.org/development/gettingstarted))
or other third-party services to interpret the lake data.
Once copied, a new service can be instantiated by pointing a `super db serve`
at the copy of the lake.

Functionality like [data compaction](../command/db-manage.md) and retention are all API-driven.

Bite-sized components are unified by the super-structured data, usually in the BSUP format:
* All lake meta-data is available via meta-queries.
* All lake operations available through the service API are also available
directly via the `super db` command.
* Lake management is agent-driven through the API.  For example, instead of complex policies
like data compaction being implemented in the core with some fixed set of
algorithms and policies, an agent can simply hit the API to obtain the meta-data
of the objects in the lake, analyze the objects (e.g., looking for too much
key space overlap) and issue API commands to merge overlapping objects
and delete the old fragmented objects, all with the transactional consistency
of the commit log.
* Components are easily tested and debugged in isolation.

## The Database Model

A SuperDB database is a cloud-native arrangement of data, optimized for search,
analytics, ETL, data discovery, and data preparation
at scale based on data represented in accordance
with the [super-structured data model](../formats/model.md).

A database is organized into a collection of data pools forming a single
administrative domain.  The current implementation supports
ACID append and delete semantics at the commit level while
we have plans to support CRUD updates at the primary-key level
in the near future.

TODO: make pools independent entities then tie them together with a separate
layer of adminstrative glue (i.e., there should be no depedencies in a pool
that are required to interpret and query it outside of the pool entity)

TODO: back off on github metaphor?

The semantics of a SuperDB database loosely follows the nomenclature and
design patterns of [`git`](https://git-scm.com/).  In this approach,
* a _lake_ is like a GitHub organization,
* a _pool_ is like a `git` repository,
* a _branch_ of a _pool_ is like a `git` branch,
* the _use_  command is like a `git checkout`, and
* the _load_ command is like a `git add/commit/push`.

A core theme of the SuperDB database design is _ergonomics_.  Given the Git metaphor,
our goal here is that the lake tooling be as easy and familiar as Git is
to a technical user.

Since databases are built upon the super-structured data model,
getting different kinds of data into and out of a lake is easy.
There is no need to define schemas or tables and then fit
semi-structured data into schemas before loading data into a lake.
And because SuperDB supports a large family of formats and the load endpoint
automatically detects most formats, it's easy to just load data into a lake
without thinking about how to convert it into the right format.

### CLI-First Approach

The SuperDB project has taken a _CLI-first approach_ to designing and implementing
the system.  Any time a new piece of functionality is added to the lake,
it is first implemented as a `super db` command.  This is particularly convenient
for testing and continuous integration as well as providing intuitive,
bite-sized chunks for learning how the system works and how the different
components come together.

While the CLI-first approach provides these benefits,
all of the functionality is also exposed through an [API](../database/api.md) to
a lake service.  Many use cases involve an application like
[SuperDB Desktop](https://zui.brimdata.io/) or a
programming environment like Python/Pandas interacting
with the service API in place of direct use with `super db`.

### Storage Layer

The lake storage model is designed to leverage modern cloud object stores
and separates compute from storage.

A lake is entirely defined by a collection of cloud objects stored
at a configured object-key prefix.  This prefix is called the _storage path_.
All of the meta-data describing the data pools, branches, commit history,
and so forth is stored as cloud objects inside of the lake.  There is no need
to set up and manage an auxiliary metadata store.

Data is arranged in a lake as a set of pools, which are comprised of one
or more branches, which consist of a sequence of data [commit objects](#commit-objects)
that point to cloud data objects.

Cloud objects and commits are immutable and named with globally unique IDs,
based on the [KSUIDs](https://github.com/segmentio/ksuid), and many
commands may reference various lake entities by their ID, e.g.,
* _Pool ID_ - the KSUID of a pool
* _Commit object ID_ - the KSUID of a commit object
* _Data object ID_ - the KSUID of a committed data object

Data is added and deleted from the lake only with new commits that
are implemented in a transactionally consistent fashion.  Thus, each
commit object (identified by its globally-unique ID) provides a completely
consistent view of an arbitrarily large amount of committed data
at a specific point in time.

While this commit model may sound heavyweight, excellent live ingest performance
can be achieved by micro-batching commits.

Because the lake represents all state transitions with immutable objects,
the caching of any cloud object (or byte ranges of cloud objects)
is easy and effective since a cached object is never invalid.
This design makes backup/restore, data migration, archive, and
replication easy to support and deploy.

The cloud objects that comprise a lake, e.g., data objects,
commit history, transaction journals, partial aggregations, etc.,
are stored as super-structured data, i.e., either as [row-based Super Binary (BSUP)](../formats/bsup.md)
or [Super Columnar (CSUP)](../formats/csup.md).
This makes introspection of the lake structure straightforward as many key
lake data structures can be queried with metadata queries and presented
to a client for further processing by downstream tooling.

The implementation also includes a storage abstraction that maps the cloud object
model onto a file system so that lakes can also be deployed on standard file systems.

### Command Personalities

The `super db` command provides a single command-line interface to SuperDB data lakes, but
different personalities are taken on by `super db` depending on the particular
sub-command executed and the database [connection](../command/db.md#database-connection).

To this end, `super db` can take on one of three personalities:

* _Direct Access_ - When the lake is a storage path (`file` or `s3` URI),
then the `super db` commands (except for `serve`) all operate directly on the
lake located at that path.
* _Client Personality_ - When the lake is an HTTP or HTTPS URL, then the
lake is presumed to be a service endpoint and the client
commands are directed to the service managing the lake.
* _Server Personality_ - When the [`super db serve`](../command/db-serve.md) command is executed, then
the personality is always the server personality and the lake must be
a storage path.  This command initiates a continuous server process
that serves client requests for the lake at the configured storage path.

Note that a storage path on the file system may be specified either as
a fully qualified file URI of the form `file://` or be a standard
file system path, relative or absolute, e.g., `/lakes/test`.

Concurrent access to any lake storage, of course, preserves
data consistency.  You can run multiple `super db serve` processes while also
running any `super db` lake command all pointing at the same storage endpoint
and the lake's data footprint will always remain consistent as the endpoints
all adhere to the consistency semantics of the lake.

>[!NOTE]
> Transactional data consistency is not fully implemented yet for
> the S3 endpoint so only single-node access to S3 is available right now,
> though support for multi-node access is forthcoming.
> For a shared file system, the close-to-open cache consistency
> semantics of [NFS](https://en.wikipedia.org/wiki/Network_File_System) should provide the
> necessary consistency guarantees needed by
> the lake though this has not been tested.  Multi-process, single-node
> access to a local file system has been thoroughly tested and should be
> deemed reliable, i.e., you can run a direct-access instance of `super db` alongside
> a server instance of `super db` on the same file system and data consistency will
> be maintained.

### Locating the Database

At times you may want `super db` commands to access the same lake storage
used by other tools such as [SuperDB Desktop](https://zui.brimdata.io/). To help
enable this by default while allowing for separate lake storage when desired,
`super db` checks each of the following in order to attempt to locate an existing
lake.

1. The contents of the `-db` option (if specified)
2. The contents of the `SUPER_DB` environment variable (if defined)
3. A lake service running locally at `http://localhost:9867` (if a socket
   is listening at that port)
4. A `super` subdirectory below a path in the
   `XDG_DATA_HOME` **TODO: add link to basedir spec**
   environment variable (if defined)
5. A default file system location based on detected OS platform:
   - `%LOCALAPPDATA%\super` on Windows
   - `$HOME/.local/share/super` on Linux and macOS

### Data Pools

A database is made up of _data pools_, which are like "collections" in NoSQL
document stores.  Pools may have one or more branches and every pool always
has a branch called `main`.

A pool is created with the [`create` command](../command/db-create.md)
and a branch of a pool is created with the [`branch` command](../command/db-branch.md).

A pool name can be any valid UTF-8 string and is allocated a unique ID
when created.  The pool can be referred to by its name or by its ID.
A pool may be renamed but the unique ID is always fixed.

#### Commit Objects

Data is added into a pool in atomic units called _commit objects_.

Each commit object is assigned a global ID.
Similar to Git, commit objects are arranged into a tree and
represent the entire commit history of the lake.

>[!NOTE]
> Technically speaking, Git can merge from multiple parents and thus
> Git commits form a directed acyclic graph instead of a tree;
> SuperDB does not currently support multiple parents in the commit object history.

A branch is simply a named pointer to a commit object in the lake
and like a pool, a branch name can be any valid UTF-8 string.
Consistent updates to a branch are made by writing a new commit object that
points to the previous tip of the branch and updating the branch to point at
the new commit object.  This update may be made with a transaction constraint
(e.g., requiring that the previous branch tip is the same as the
commit object's parent); if the constraint is violated, then the transaction
is aborted.

The _working branch_ of a pool may be selected on any command with the `-use` option
or may be persisted across commands with the [`use` command](../command/db-use.md) so that
`-use` does not have to be specified on each command-line.  For interactive
workflows, the `use` command is convenient but for automated workflows
in scripts, it is good practice to explicitly specify the branch in each
command invocation with the `-use` option.

#### Commitish

Many `super db` commands operate with respect to a commit object.
While commit objects are always referenceable by their commit ID, it is also convenient
to refer to the commit object at the tip of a branch.

The entity that represents either a commit ID or a branch is called a _commitish_.
A commitish is always relative to the pool and has the form:
* `<pool>@<id>` or
* `<pool>@<branch>`

where `<pool>` is a pool name or pool ID, `<id>` is a commit object ID,
and `<branch>` is a branch name.

In particular, the working branch set by the [`use` command](../command/db-use.md) is a commitish.

A commitish may be abbreviated in several ways where the missing detail is
obtained from the working-branch commitish, e.g.,
* `<pool>` - When just a pool name is given, then the commitish is assumed to be
`<pool>@main`.
* `@<id>` or `<id>`- When an ID is given (optionally with the `@` prefix), then the commitish is assumed to be `<pool>@<id>` where `<pool>` is obtained from the working-branch commitish.
* `@<branch>` - When a branch name is given with the `@` prefix, then the commitish is assumed to be `<pool>@<id>` where `<pool>` is obtained from the working-branch commitish.

An argument to a command that takes a commit object is called a _commitish_
since it can be expressed as a branch or as a commit ID.

#### Pool Key

Each data pool is organized according to its configured _pool key_,
which is the sort key for all data stored in the lake.  Different data pools
can have different pool keys but all of the data in a pool must have the same
pool key.

As pool data is often comprised of [records](../formats/model.md#21-record) (analogous to JSON objects),
the pool key is typically a field of the stored records.
When pool data is not structured as records/objects (e.g., scalar or arrays or other
non-record types), then the pool key would typically be configured
as the [special value `this`](../super-sql/intro.md#pipe-scoping).

Data can be efficiently scanned if a query has a filter operating on the pool
key.  For example, on a pool with pool key `ts`, the query `ts == 100`
will be optimized to scan only the data objects where the value `100` could be
present.

>[!NOTE]
> The pool key will also serve as the primary key for the forthcoming
> CRUD semantics.

A pool also has a configured sort order, either ascending or descending
and data is organized in the pool in accordance with this order.
Data scans may be either ascending or descending, and scans that
follow the configured order are generally more efficient than
scans that run in the opposing order.

Scans may also be range-limited but unordered.

Any data loaded into a pool that lacks the pool key is presumed
to have a null value with regard to range scans.  If large amounts
of such "keyless data" are loaded into a pool, the ability to
optimize scans over such data is impaired.

### Time Travel

Because commits are transactional and immutable, a query
sees its entire data scan as a fixed "snapshot" with respect to the
commit history.  In fact, the [`from` operator](../super-sql/operators/from.md)
allows a commit object to be specified with the `@` suffix to a
pool reference, e.g.,
```
super db query 'from logs@1tRxi7zjT7oKxCBwwZ0rbaiLRxb | ...'
```
In this way, a query can time-travel through the commit history.  As long as the
underlying data has not been deleted, arbitrarily old snapshots of the
lake can be easily queried.

If a writer commits data after or while a reader is scanning, then the reader
does not see the new data since it's scanning the snapshot that existed
before these new writes occurred.

Also, arbitrary metadata can be [committed to the log](../command/db-load.md),
e.g., to associate derived analytics to a specific
journal commit point potentially across different data pools in
a transactionally consistent fashion.

While time travel through commit history provides one means to explore
past snapshots of the commit history, another means is to use a timestamp.
Because the entire history of branch updates is stored in a transaction journal
and each entry contains a timestamp, branch references can be easily
navigated by time.  For example, a list of branches of a pool's past
can be created by scanning the internal "pools log" and stopping at the largest
timestamp less than or equal to the desired timestamp.  Then using that
historical snapshot of the pools, a branch can be located within the pool
using that pool's "branches log" in a similar fashion, then its corresponding
commit object can be used to construct the data of that branch at that
past point in time.

>[!NOTE]
> Time travel using timestamps is a forthcoming feature.
