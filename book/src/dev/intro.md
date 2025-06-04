# Developer

TODO: rework this

## Design Philosophy

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

Functionality like [data compaction](../commands/super-db.md#manage) and retention are all API-driven.

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
