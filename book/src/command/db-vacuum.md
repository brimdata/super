### Command

&emsp; **vacuum** &mdash; vacuum deleted storage in database

### Synopsis

```
super db vacuum [ options ]
```

### Options

* `-dryrun` run vacuum without deleting anything
* `-f` do not prompt for confirmation
* `-use` specify commit to use, i.e., pool, pool@branch, or pool@commit

### Description

The `vacuum` command permanently removes underlying data objects that have
previously been subject to a [`delete`](db-delete.md) operation.  As this is a
DANGER ZONE command, you must confirm that you want to remove
the objects to proceed.  The `-f` option can be used to force removal
without confirmation.  The `-dryrun` option may also be used to see a summary
of how many objects would be removed by a `vacuum` but without removing them.
