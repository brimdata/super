# `zq`

`zq` is a command-line tool to search, analyze, and transform structured logs. 
 It evaluates [ZQL ](../../zql/docs/README.md) queries against input log
  files, producing an output log stream in the [ZNG](../../zng/docs/spec.md)
  format by default.
  
For all `zq` options, use the help subcommand:

```
zq help
```

## Examples

Here are a few examples using a small Zeek formatted log file, `conn.log`,
located in this directory. See the
[zq-sample-data repo](https://github.com/brimsec/zq-sample-data) for more test
data, which is used in the examples in the
[query language documentation](../../zql/docs/README.md).

To cut the columns of a Zeek "conn" log like `zeek-cut`, and output to the
 terminal, use `cut`:

```
zq -t "* | cut ts,id.orig_h,id.orig_p" conn.log
```

The `-t` tells `zq` to use [TZNG](../../zng/docs/spec.md#4-zng-text-format-tzng)
for its output format. The "`*`" 
tells `zq` to match every line, which is sent to the `cut` processor
using the UNIX-like pipe syntax.

When looking over everything like this, you can omit the search pattern
as a shorthand.
```
zq -t "cut ts,id.orig_h,id.orig_p" conn.log
```

The default output is the binary ZNG format. If you want just the tab-separated
 lines like `zeek-cut`, you can specify text output.
```
zq -f text "cut ts,id.orig_h,id.orig_p" conn.log
```
If you want the old-style Zeek [ASCII TSV](https://docs.zeek.org/en/master/log-formats.html#zeek-tsv-format-logs)
log format, use the `-f` flag specifying `zeek` for the output
format:
```
zq -f zeek "cut ts,id.orig_h,id.orig_p" conn.log
```
You can use an aggregate function to summarize data over one or
more fields, e.g., summing field values, counting, or computing an average.
```
zq -t "sum(orig_bytes)" conn.log
zq -t "orig_bytes > 10000 | count()" conn.log
zq -t "avg(orig_bytes)" conn.log
```

The [ZNG specification](../../zng/docs/spec.md) describes how the format can
represent a stream of heterogeneously typed records. By leveraging this,
diverse Zeek logs can be combined into a single file.

```
zq *.log > all.zng
```

### Comparisons

The following usage of `cut` (repeated from above):

```
zq -f text "cut ts,id.orig_h,id.orig_p" conn.log
```

is functionally equivalent to this `zeek-cut` command:

```
zeek-cut ts id.orig_h id.orig_p < conn.log
```

If your Zeek events are stored as JSON, the equivalent `jq` command is:

```
jq -c '. | { ts, "id.orig_h", "id.orig_p" }' conn.ndjson
```

Comparisons of other simple operations and their relative performance are described
at the [performance](../../performance/README.md) page.


## Formats

| Format | Read | Auto-Detect | Write | Description |
|--------|------|-------------|-------|-------------|
| zng | yes | yes | yes | [ZNG specification](../../zng/docs/spec.md) |
| tzng | yes | yes | yes | [TZNG specification](../../zng/docs/spec.md#4-zng-text-format-tzng) |
| ndjson | yes | yes | yes | Newline delimited JSON records |
| zeek  | yes | yes | yes | [Zeek compatible](https://docs.zeek.org/en/master/log-formats.html#zeek-tsv-format-logs) tab separated values |
| zjson | yes | yes | yes | [ZNG over JSON](../../zng/docs/zng-over-json.md) |
| parquet | yes | no | no | [Parquet file format](https://github.com/apache/parquet-format#file-format)
| table | no | no | yes | table output, with column headers |
| text | no | no | yes | space separated output |
| types | no | no | yes | outputs input record types |
