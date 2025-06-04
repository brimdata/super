### Function

&emsp; **every** &mdash; bucket `ts` using a duration

### Synopsis

```
every(d: duration) -> time
```

### Description

The _every_ function is a shortcut for `bucket(ts, d)`.
This provides a convenient binning function for aggregations
when analyzing time-series data like logs that have a `ts` field.

### Examples

Operate on a sequence of times:
```mdtest-spq
# spq
values {ts,val:0},{ts:ts+1s},{ts:ts+2h2s}
| values every(1h)
| sort
# input
{ts:2021-02-01T12:00:01Z}
# expected output
2021-02-01T12:00:00Z
2021-02-01T12:00:00Z
2021-02-01T14:00:00Z
```

Use as a grouping key:
```mdtest-spq
# spq
values {ts,val:1},{ts:ts+1s,val:2},{ts:ts+2h2s,val:5}
| sum(val) by every(1h)
| sort
# input
{ts:2021-02-01T12:00:01Z}
# expected output
{ts:2021-02-01T12:00:00Z,sum:3}
{ts:2021-02-01T14:00:00Z,sum:5}
```
