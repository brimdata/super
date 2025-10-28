### Command

&emsp; **create** &mdash; create a new pool in a database

### Synopsis

```
super db create [-orderby key[,key...][:asc|:desc]] <name>
```

### Options

TODO

Additional options of the [db sub-command](db.md#options)

### Description

The `create` command creates a new data pool with the given name,
which may be any valid UTF-8 string.

The `-orderby` option indicates the [pool key](#pool-key) that is used to sort
the data in the pool, which may be in ascending or descending order.

If a pool key is not specified, then it defaults to
the [special value `this`](../language/pipeline-model.md#the-special-value-this).

A newly created pool is initialized with a branch called `main`.

> Pools can be used without thinking about branches.  When referencing a pool without
> a branch, the tooling presumes the "main" branch as the default, and everything
> can be done on main without having to think about branching.
