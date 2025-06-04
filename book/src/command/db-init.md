### Command

&emsp; **init** &mdash; create and initialize a new database

### Synopsis

```
super db init [path]
```

### Options

TODO

Additional options of the [db sub-command](db.md#options)

### Description

A new database is created and initialized with the `init` command.  The `path` argument
is a [storage path](../database/intro.md#storage-layer)
and is optional.  If not present, the path
is [determined automatically](db.md#database-connection).

If the database already exists, `init` reports an error and does nothing.

Otherwise, the `init` command writes the initial cloud objects to the
storage path to create a new, empty database at the specified path.
