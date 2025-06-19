---
weight: 2
title: zed
heading: zed Tutorial
---

`super` is great, but what if we have a lot of data on which we want to perform search and
analytics? This is where the `zed` command comes in. `zed` builds on the type
system and language found in `super` and adds a high performance data lake on top.

> Note: `zed` is currently in alpha form. Check out its current status in the
> [`super db` command](../../commands/super-db.md) documentation..

## Creating a Lake

We start by creating our Zed lake. First we'll set the `ZED_LAKE`
environment variable that tells `zed` where we want to store our lake:

```bash
$ export ZED_LAKE=$HOME/.zedlake
```

Next we instruct `zed` to initialize our lake:

```bash
$ zed init
```
=>
```
lake created: /path/to/home/.zedlake
```

## Adding data to our lake

Let's add some data.

Data is stored in pools in a Zed lake. You might say a pool is similar to a
table in a SQL database except unlike a SQL table a Zed pool has no schema to
which underlying data must adhere. Any data is welcome in a Zed pool! A Zed
pool does have a pool key (or field) by which data is sorted. You might think of
a pool key as a pool's primary index. Though individual values in a pool are not
required to have the pool key field, it is nice to have a pool key that fits the
data since this will allow Zed to efficiently query data within a range of the
pool key without having to touch the entire data set.

For this primer we'll work with pull requests on this public repository via the
[GitHub API](https://docs.github.com/en/rest/pulls/pulls#list-pull-requests).
Let's create a pool to store this data and use the field `created_at` as the
pool key, sorted in descending order:

```bash
$ zed create -orderby created_at:desc prs
```
=>
```
pool created: prs <unique pool ID>
```

Using `zed ls` we can view all the pools in the lake:

```bash
$ zed ls
```
=>
```
prs <pool_id> key created_at order desc
```

Let's add some pull request data I've prefetched from the GitHub API
[here](github1.bsup):

```bash
$ zed load -use prs github1.bsup
```
=>
```
<commit_id> committed
```

Our data has been committed. The `-use prs` argument in `zed load` tells
`zed` to load our data into the `prs` pool.

## Querying our data

With our data now loaded let's run a quick `count()` query to verify that we have
the expected data. To do this we'll use the `zed query` command. To those
familiar with [`super`](../../commands/super.md), `zed query` operates similarly except
it doesn't accept file input arguments since it queries pools.

```bash
$ zed query -use prs 'count()'
```
=>
```
{count:100(uint64)}
```

This looks good so far, but let's do something more interesting. First let's use
the `zed use` command to set `prs` as our default pool so we don't have to type
the `-use` argument every time we operate on this pool.

```bash
$ zed use prs
```

We can run an aggregation to see who has created the most PRs during the time range
of this first data set:

```bash
$ zed query 'count() by user:=user.login | sort count desc'
```
=>
```
{user:"mccanne",count:40(uint64)}
{user:"mattnibs",count:23(uint64)}
{user:"aswan",count:20(uint64)}
{user:"henridf",count:9(uint64)}
{user:"nwt",count:5(uint64)}
{user:"philrz",count:3(uint64)}
```

A productive few weeks for McCanne!

We can use the `min` and `max` aggregations to see the time range of our data set:

```bash
$ zed query -S 'min(created_at), max(created_at)'
```
=>
```
{
    min: 2019-11-11T19:50:46Z,
    max: 2019-12-05T16:56:57Z
}
```

That's not a lot of data, so let's add some more.

## Adding additional data

Additional data can be added to our pool by running `zed load` on our second
[data set](github2.bsup):

```bash
$ zed load github2.bsup
```

Running our `min(created_at), max(created_at)` query, we'll see that we now have
almost two years of pull requests:

```bash
$ zed query -S 'min(created_at), max(created_at)'
```
=>
```
{
    min: 2019-11-11T19:50:46Z,
    max: 2021-09-19T19:31:43Z
}
```

Now let's run a bucketed aggregation to count approximate PRs per month (specifically, PRs
bucketed in 12 equal spans of a year):

```
$ zed query 'count() by ts:=bucket(created_at, 1y/12) | sort ts'
```
=>
```
{ts:2019-10-20T04:00:00Z,count:28(uint64)}
{ts:2019-11-19T14:00:00Z,count:123(uint64)}
{ts:2019-12-20T00:00:00Z,count:72(uint64)}
{ts:2020-01-19T10:00:00Z,count:102(uint64)}
{ts:2020-02-18T20:00:00Z,count:114(uint64)}
{ts:2020-03-20T06:00:00Z,count:111(uint64)}
{ts:2020-04-19T16:00:00Z,count:137(uint64)}
{ts:2020-05-20T02:00:00Z,count:74(uint64)}
...
```

There are lots of PRs that happened in the ~30 day block starting on 4/19/2020, so let's zoom in here
and see who created these PRs:

```
$ zed query 'from prs range 2020-04-19T16:00:00Z to 2020-05-20T02:00:00Z
             | count() by user:=user.login | sort count desc'
```
=>
```
{user:"mccanne",count:35(uint64)}
{user:"henridf",count:34(uint64)}
{user:"aswan",count:27(uint64)}
{user:"mattnibs",count:14(uint64)}
{user:"alfred-landrum",count:12(uint64)}
{user:"philrz",count:9(uint64)}
{user:"mikesbrown",count:5(uint64)}
{user:"nwt",count:1(uint64)}
```

McCanne is once again in the lead but Henri is not far behind.

The important thing demonstrated in the above query is the use of the `from`
operator. The `from` operator specifies to query the `main` branch of the `prs` pool
and also defines a time range for the query. The range part of the query is an
important distinction from `super`. Whereas `super` would be required to
scan the entire data set to execute this query, this Zed pool which stores data
sorted by `created_at` can skip all data that doesn't fall within the range
`2020-04-19T16:00:00Z to 2020-05-20T02:00:00Z`. This results in a much faster
query over the limited range.

## Time travel

Suppose we made a mistake by loading the last chunk of data.
Perhaps we applied the wrong transform to the incoming data. Is there any
way we can fix this? Similar to version control systems like [`git`](https://git-scm.com),
a Zed lake maintains a linear history (or commit log) of all the changes made to
a pool. There are many advantages to having data stored in this manner, one of
which is that we can easily discard changes we don't want.

First we'll use `zed log` command to view the history of commits (IDs will vary in your output):

```
$ zed log
```
=>
```
commit 26i2N0uu6wEo5XAhPMid6eQsamF
Author: nibs@Matthews-MacBook-Air-2.local
Date:   2022-03-21T26:03:25Z

    loaded 1 data object

    26i2MyhTem11tTOS2HSa1cgnYyz 1900 records in 765024 data bytes

commit 26i2MeIlGMoGHzjpbZttKtUuSFb
Author: nibs@Matthews-MacBook-Air-2.local
Date:   2022-03-21T19:47:19Z

    loaded 1 data object

    26i2Mi5xPdaTRxbho05DUhTYHIx 100 records in 46000 data bytes
```

Let's revert the most recent commit:

```
zed revert 26i2N0uu6wEo5XAhPMid6eQsamF
```
=>
```
"main": 26i2N0uu6wEo5XAhPMid6eQsamF reverted in 26nY9AYOxx2WtSfKGjof9R2MOYb
```

We can run `count()` to see we're back to our original 100 values.

```bash
$ zed query 'count()'
```
=>
```
{count:100(uint64)}
```

If we made a mistake and we'd like to keep the data, we can also revert our
revert commit:

```
$ zed revert 26nY9AYOxx2WtSfKGjof9R2MOYb
```

Running `count()` will show we're back to 2000 values:

```bash
$ zed query 'count()'
```
=>
```
{count:2000(uint64)}
```

## Running as a service

Now that we've compiled an interesting data set, how might we share this with
others? Using the `zed serve` command we can launch our Zed lake as a service
that will allow multiple clients to query and add data to the same lake. In a
separate console window run:

```
$ zed serve -db $HOME/.zedlake
```
=>
```
{"level":"info","ts":1647957396.828584,"msg":"Open files limit raised","limit":10240}
{"level":"info","ts":1647957396.8318028,"logger":"core","msg":"Started"}
{"level":"info","ts":1647957396.83288,"logger":"httpd","msg":"Listening","addr":"[::]:9867"}
```

We now have a service running on `http://localhost:9867`. If we set the
`ZED_LAKE` environment variable we defined at the beginning to this URL we can
run the full set of `zed` commands against this service:

```
$ export ZED_LAKE=http://localhost:9867
$ zed query -S 'min(created_at), max(created_at)'
```
=>
```
{
    min: 2019-11-11T19:50:46Z,
    max: 2021-08-10T19:48:56Z
}
```

## Where to go from here?

Obviously this is only the tip of the iceberg in terms of things that can be done with
the `zed` command. Some suggested next steps:

1. Dig deeper into SuperDB data lakes by having a look at the [`super db` command](../../commands/super-db.md) documentation.
2. Get a better idea of ways you can query your data by looking at the
[Zed language documentation](../../language/_index.md).

If you have any questions or run into any snags, join the friendly Zed community
at the [Brim Data Slack workspace](https://www.brimdata.io/join-slack/).
