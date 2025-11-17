### Command

&emsp; **merge** &mdash; merged data from one branch to another

### Synopsis

```
super db merge -use logs@updates <branch>
```

### Options

Additional options of the [db sub-command](db.md#options)

### Description

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
