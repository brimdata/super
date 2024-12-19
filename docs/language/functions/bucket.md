### Function

&emsp; **bucket** &mdash; quantize a time or duration value into buckets of equal time spans

### Synopsis

```
bucket(val: time, span: duration|number) -> time
bucket(val: duration, span: duration|number) -> duration
```

### Description

The _bucket_ function quantizes a time or duration `val`
(or value that can be coerced to time) into buckets that
are equally spaced as specified by `span` where the bucket boundary
aligns with 0.

### Examples

Bucket a couple times to hour intervals:
```mdtest-command
echo '2020-05-26T15:27:47Z "5/26/2020 3:27pm"' |
  super -z -c 'yield bucket(time(this), 1h)' -
```

```mdtest-output
2020-05-26T15:00:00Z
2020-05-26T15:00:00Z
```
