### Command

&emsp; **delete** &mdash; delete data from a pool

### Synopsis

```
super db delete [options] <id> [<id>...]
super db delete [options] -where <filter>
```

### Options

TODO

Additional options of the [db sub-command](db.md#options)

### Description

The `delete` command removes one or more data objects indicated by their ID from a pool.
This command
simply removes the data from the branch without actually deleting the
underlying data objects thereby allowing time travel to work in the face
of deletes.  Permanent deletion of underlying data objects is handled by the
separate [`vacuum`](db-vacuum.md) command.

If the `-where` flag is specified, delete will remove all values for which the
provided filter expression is true.  The value provided to `-where` must be a
single filter expression, e.g.:

```
super db delete -where 'ts > 2022-10-05T17:20:00Z and ts < 2022-10-05T17:21:00Z'
```
