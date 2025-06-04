### Command

&emsp; **ls** &mdash; list the pools in a database

### Synopsis

```
super db ls [options] [pool]
```

### Options

Additional options of the [db sub-command](db.md#options)

### Description

The `ls` command lists pools in a database or branches in a pool.

By default, all pools in the database are listed along with each pool's unique ID
and [sort key](db.md#sort-key).

If a pool name or pool ID is given, then the pool's branches are listed along
with the ID of their commit object, which points at the tip of each branch.
