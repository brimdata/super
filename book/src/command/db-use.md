### Command

&emsp; **use** &mdash; set working branch for `db` commands

### Synopsis

```
super db use [ <commitish> ]
```

### Options

* `-use` specify commit to use, i.e., pool, pool@branch, or pool@commit

### Description

The `use` command sets the working branch to the indicated commitish.
When run with no argument, it displays the working branch and
[database connection](db.md#database-connection).

For example,
```
super db use logs
```
provides a "pool-only" commitish that sets the working branch to `logs@main`.

If a `@branch` or commit ID are given without a pool prefix, then the pool of
the commitish previously in use is presumed.  For example, if you are on
`logs@main` then run this command:
```
super db use @test
```
then the working branch is set to `logs@test`.

To specify a branch in another pool, simply prepend
the pool name to the desired branch:
```
super db use otherpool@otherbranch
```
This command stores the working branch in `$HOME/.super_head`.
