### Command

&emsp; **branch** &mdash; create a new branch on a pool

### Synopsis

```
super db branch [options] [name]
```

### Options

TODO

Additional options of the [db sub-command](db.md#options)

### Description

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
