### Command

&emsp; **drop** &mdash; remove a pool from a database

### Synopsis

```
super db drop [options] <name>|<id>
```

### Options

TODO

Additional options of the [db sub-command](db.md#options)

### Description

The `drop` command deletes a pool and all of its constituent data.
As this is a DANGER ZONE command, you must confirm that you want to delete
the pool to proceed.  The `-f` option can be used to force the deletion
without confirmation.
