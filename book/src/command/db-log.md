### Command

&emsp; **log** &mdash; display the commit log

### Synopsis

```
super db log [options] [commitish]
```

### Options

Additional options of the [db sub-command](db.md#options)

### Description

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

> The branchlog meta-query source is not yet implemented.
