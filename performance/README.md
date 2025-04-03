# Performance

The tables below provide a summary of simple operations and how `super`
performs at them relative to `zeek-cut` and `jq`. All operations were performed
on an AWS `t3.2xlarge` VM (8 vCPUs, 32 GB memory, 30 GB gp2 SSD).
`make perf-compare` was used to generate the results.

As there are many results to sift through, here's a few key summary take-aways:

* The numerous input/output formats in `super` are helpful for fitting into your
legacy pipelines. However, Super Binary performs the best of all `super`-compatible
formats, due to its binary/optimized nature. If you have logs in a text-based
format and expect to query them many times, a one-time pass through `super` to
convert them to Super Binary format will save you significant time.

* Despite it having some CPU cost, the LZ4 compression that `super` performs by
default when outputting Super Binary is shown to have a negligible user-perceptible
performance impact. With this sample data, the LZ4-compressed Super Binary is less than
half the size as when uncompressed.

* Particularly when working in Super Binary format and when simple analytics (counting,
grouping) are in play, `super` can significantly outperform `jq`. That said, `super`
does not (yet) include the full set of mathematical/other operations available
in `jq`. If there's glaring functional omissions that are limiting your use of
`super`, we welcome [contributions](../README.md#contributing).

* For the permutations with `json` input the recommended approach for
[shaping Zeek JSON](https://zed.brimdata.io/docs/integrations/zeek/shaping-zeek-ndjson)
was followed as the input data was being read. In addition to conforming to the
best practices as described in that article, this also avoids a problem
described in [a comment in super/2123](https://github.com/brimdata/super/pull/2123#issuecomment-859164320).
Separate tests on our VM confirmed the shaping portion of the runs with JSON
input consumed approximately 5 seconds out of the total run time on each of
these runs.

# Results

The results below reflect performance as of `super` commit `baf921f`.

### Output all events unmodified

|**<br>Tool**|**<br>Arguments**|**Input<br>Format**|**Output<br>Format**|**<br>Real**|**<br>User**|**<br>Sys**|
|:----------:|:---------------:|:-----------------:|:------------------:|-----------:|-----------:|----------:|
|`super`|`*`|zeek|zeek|10.23|11.29|0.26|
|`super`|`*`|zeek|bsup|3.85|3.95|0.08|
|`super`|`*`|zeek|bsup-uncompressed|3.21|3.23|0.06|
|`super`|`*`|zeek|sup|23.11|25.46|0.54|
|`super`|`*`|zeek|json|27.63|28.70|0.39|
|`super`|`*`|bsup|zeek|6.63|8.27|0.20|
|`super`|`*`|bsup|bsup|1.16|2.21|0.10|
|`super`|`*`|bsup|bsup-uncompressed|1.24|1.56|0.08|
|`super`|`*`|bsup|sup|18.15|20.69|0.42|
|`super`|`*`|bsup|json|23.83|25.90|0.36|
|`super`|`*`|bsup-uncompressed|zeek|6.59|8.31|0.20|
|`super`|`*`|bsup-uncompressed|bsup|1.28|2.19|0.08|
|`super`|`*`|bsup-uncompressed|bsup-uncompressed|1.24|1.40|0.07|
|`super`|`*`|bsup-uncompressed|sup|19.39|22.21|0.44|
|`super`|`*`|bsup-uncompressed|json|23.67|25.73|0.37|
|`super`|`*`|sup|zeek|156.14|176.34|3.79|
|`super`|`*`|sup|bsup|147.29|163.52|3.02|
|`super`|`*`|sup|bsup-uncompressed|150.17|167.85|3.37|
|`super`|`*`|sup|sup|169.48|190.82|3.80|
|`super`|`*`|sup|json|183.59|204.98|4.49|
|`super`|`*`|json|zeek|28.25|80.26|4.44|
|`super`|`*`|json|bsup|26.04|66.42|3.30|
|`super`|`*`|json|bsup-uncompressed|27.44|68.50|3.62|
|`super`|`*`|json|sup|33.52|105.72|5.23|
|`super`|`*`|json|json|35.29|103.72|4.71|
|`zeek-cut`||zeek|zeek-cut|1.40|1.42|0.22|
|`jq`|`-c '.'`|json|json|33.35|36.50|1.83|

### Extract the field `ts`

|**<br>Tool**|**<br>Arguments**|**Input<br>Format**|**Output<br>Format**|**<br>Real**|**<br>User**|**<br>Sys**|
|:----------:|:---------------:|:-----------------:|:------------------:|-----------:|-----------:|----------:|
|`super`|`cut quiet(ts)`|zeek|zeek|8.73|12.42|1.15|
|`super`|`cut quiet(ts)`|zeek|bsup|7.25|10.53|0.96|
|`super`|`cut quiet(ts)`|zeek|bsup-uncompressed|7.09|10.38|0.98|
|`super`|`cut quiet(ts)`|zeek|sup|8.79|12.54|1.11|
|`super`|`cut quiet(ts)`|zeek|json|8.98|12.92|0.91|
|`super`|`cut quiet(ts)`|bsup|zeek|1.94|3.51|0.17|
|`super`|`cut quiet(ts)`|bsup|bsup|1.19|2.22|0.13|
|`super`|`cut quiet(ts)`|bsup|bsup-uncompressed|1.22|2.10|0.16|
|`super`|`cut quiet(ts)`|bsup|sup|2.13|3.58|0.18|
|`super`|`cut quiet(ts)`|bsup|json|2.16|3.62|0.13|
|`super`|`cut quiet(ts)`|bsup-uncompressed|zeek|1.87|3.40|0.13|
|`super`|`cut quiet(ts)`|bsup-uncompressed|bsup|1.51|2.45|0.11|
|`super`|`cut quiet(ts)`|bsup-uncompressed|bsup-uncompressed|1.44|2.27|0.12|
|`super`|`cut quiet(ts)`|bsup-uncompressed|sup|2.25|3.86|0.21|
|`super`|`cut quiet(ts)`|bsup-uncompressed|json|2.06|3.54|0.17|
|`super`|`cut quiet(ts)`|sup|zeek|155.19|177.80|4.53|
|`super`|`cut quiet(ts)`|sup|bsup|152.61|172.17|3.80|
|`super`|`cut quiet(ts)`|sup|bsup-uncompressed|154.97|177.34|4.86|
|`super`|`cut quiet(ts)`|sup|sup|157.22|179.82|4.92|
|`super`|`cut quiet(ts)`|sup|json|166.79|191.20|5.80|
|`super`|`cut quiet(ts)`|json|zeek|31.16|77.11|4.51|
|`super`|`cut quiet(ts)`|json|bsup|30.06|73.06|3.90|
|`super`|`cut quiet(ts)`|json|bsup-uncompressed|32.17|77.60|4.52|
|`super`|`cut quiet(ts)`|json|sup|31.38|78.03|4.69|
|`super`|`cut quiet(ts)`|json|json|30.56|76.57|4.81|
|`zeek-cut`|`ts`|zeek|zeek-cut|1.53|1.51|0.23|
|`jq`|`-c '. \| { ts }'`|json|json|21.37|24.25|1.51|

### Count all events

|**<br>Tool**|**<br>Arguments**|**Input<br>Format**|**Output<br>Format**|**<br>Real**|**<br>User**|**<br>Sys**|
|:----------:|:---------------:|:-----------------:|:------------------:|-----------:|-----------:|----------:|
|`super`|`count:=count()`|zeek|zeek|3.20|3.34|0.06|
|`super`|`count:=count()`|zeek|bsup|3.21|3.26|0.09|
|`super`|`count:=count()`|zeek|bsup-uncompressed|2.98|2.99|0.06|
|`super`|`count:=count()`|zeek|sup|3.15|3.16|0.05|
|`super`|`count:=count()`|zeek|json|2.97|2.98|0.04|
|`super`|`count:=count()`|bsup|zeek|1.15|1.38|0.09|
|`super`|`count:=count()`|bsup|bsup|1.10|1.35|0.06|
|`super`|`count:=count()`|bsup|bsup-uncompressed|1.09|1.33|0.07|
|`super`|`count:=count()`|bsup|sup|1.62|1.86|0.13|
|`super`|`count:=count()`|bsup|json|1.26|1.52|0.08|
|`super`|`count:=count()`|bsup-uncompressed|zeek|1.26|1.39|0.11|
|`super`|`count:=count()`|bsup-uncompressed|bsup|1.30|1.46|0.08|
|`super`|`count:=count()`|bsup-uncompressed|bsup-uncompressed|1.37|1.51|0.10|
|`super`|`count:=count()`|bsup-uncompressed|sup|1.38|1.53|0.08|
|`super`|`count:=count()`|bsup-uncompressed|json|1.24|1.40|0.07|
|`super`|`count:=count()`|sup|zeek|159.03|178.35|3.85|
|`super`|`count:=count()`|sup|bsup|161.73|184.81|5.41|
|`super`|`count:=count()`|sup|bsup-uncompressed|161.83|181.70|4.36|
|`super`|`count:=count()`|sup|sup|157.25|178.62|4.89|
|`super`|`count:=count()`|sup|json|158.86|179.40|4.23|
|`super`|`count:=count()`|json|zeek|33.22|78.02|5.04|
|`super`|`count:=count()`|json|bsup|30.42|72.83|4.30|
|`super`|`count:=count()`|json|bsup-uncompressed|29.74|72.31|3.97|
|`super`|`count:=count()`|json|sup|29.77|72.57|4.15|
|`super`|`count:=count()`|json|json|31.86|76.31|4.67|
|`jq`|`-c -s '. \| length'`|json|json|29.21|29.24|5.03|

### Count all events, grouped by the field `id.orig_h`

|**<br>Tool**|**<br>Arguments**|**Input<br>Format**|**Output<br>Format**|**<br>Real**|**<br>User**|**<br>Sys**|
|:----------:|:---------------:|:-----------------:|:------------------:|-----------:|-----------:|----------:|
|`super`|`count() by quiet(id.orig_h)`|zeek|zeek|3.25|3.41|0.09|
|`super`|`count() by quiet(id.orig_h)`|zeek|bsup|3.38|3.48|0.10|
|`super`|`count() by quiet(id.orig_h)`|zeek|bsup-uncompressed|3.19|3.49|0.13|
|`super`|`count() by quiet(id.orig_h)`|zeek|sup|3.23|3.37|0.15|
|`super`|`count() by quiet(id.orig_h)`|zeek|json|3.33|3.51|0.24|
|`super`|`count() by quiet(id.orig_h)`|bsup|zeek|1.34|1.93|0.18|
|`super`|`count() by quiet(id.orig_h)`|bsup|bsup|1.30|1.86|0.14|
|`super`|`count() by quiet(id.orig_h)`|bsup|bsup-uncompressed|1.05|1.52|0.09|
|`super`|`count() by quiet(id.orig_h)`|bsup|sup|1.17|1.61|0.10|
|`super`|`count() by quiet(id.orig_h)`|bsup|json|1.27|1.80|0.17|
|`super`|`count() by quiet(id.orig_h)`|bsup-uncompressed|zeek|1.57|1.94|0.11|
|`super`|`count() by quiet(id.orig_h)`|bsup-uncompressed|bsup|1.62|2.05|0.11|
|`super`|`count() by quiet(id.orig_h)`|bsup-uncompressed|bsup-uncompressed|1.38|1.89|0.17|
|`super`|`count() by quiet(id.orig_h)`|bsup-uncompressed|sup|1.47|1.85|0.14|
|`super`|`count() by quiet(id.orig_h)`|bsup-uncompressed|json|1.57|2.06|0.14|
|`super`|`count() by quiet(id.orig_h)`|sup|zeek|151.64|169.93|3.48|
|`super`|`count() by quiet(id.orig_h)`|sup|bsup|154.16|172.46|3.35|
|`super`|`count() by quiet(id.orig_h)`|sup|bsup-uncompressed|157.09|175.74|3.50|
|`super`|`count() by quiet(id.orig_h)`|sup|sup|154.70|173.75|3.54|
|`super`|`count() by quiet(id.orig_h)`|sup|json|154.02|172.52|3.40|
|`super`|`count() by quiet(id.orig_h)`|json|zeek|29.41|72.86|4.27|
|`super`|`count() by quiet(id.orig_h)`|json|bsup|30.53|73.71|4.30|
|`super`|`count() by quiet(id.orig_h)`|json|bsup-uncompressed|29.27|71.83|4.26|
|`super`|`count() by quiet(id.orig_h)`|json|sup|29.65|73.21|4.23|
|`super`|`count() by quiet(id.orig_h)`|json|json|29.87|73.57|4.08|
|`jq`|`-c -s 'group_by(."id.orig_h")[] \| length as $l \| .[0] \| .count = $l \| {count,"id.orig_h"}'`|json|json|32.30|32.51|3.98|

### Output all events with the field `id.resp_h` set to `52.85.83.116`

|**<br>Tool**|**<br>Arguments**|**Input<br>Format**|**Output<br>Format**|**<br>Real**|**<br>User**|**<br>Sys**|
|:----------:|:---------------:|:-----------------:|:------------------:|-----------:|-----------:|----------:|
|`super`|`id.resp_h==52.85.83.116`|zeek|zeek|3.22|3.23|0.06|
|`super`|`id.resp_h==52.85.83.116`|zeek|bsup|3.42|3.43|0.06|
|`super`|`id.resp_h==52.85.83.116`|zeek|bsup-uncompressed|3.50|3.53|0.06|
|`super`|`id.resp_h==52.85.83.116`|zeek|sup|3.49|3.52|0.06|
|`super`|`id.resp_h==52.85.83.116`|zeek|json|3.60|3.90|0.11|
|`super`|`id.resp_h==52.85.83.116`|bsup|zeek|1.33|1.66|0.09|
|`super`|`id.resp_h==52.85.83.116`|bsup|bsup|1.19|1.49|0.08|
|`super`|`id.resp_h==52.85.83.116`|bsup|bsup-uncompressed|1.23|1.56|0.11|
|`super`|`id.resp_h==52.85.83.116`|bsup|sup|1.24|1.54|0.10|
|`super`|`id.resp_h==52.85.83.116`|bsup|json|1.14|1.45|0.07|
|`super`|`id.resp_h==52.85.83.116`|bsup-uncompressed|zeek|1.46|1.65|0.09|
|`super`|`id.resp_h==52.85.83.116`|bsup-uncompressed|bsup|1.41|1.61|0.09|
|`super`|`id.resp_h==52.85.83.116`|bsup-uncompressed|bsup-uncompressed|1.35|1.53|0.11|
|`super`|`id.resp_h==52.85.83.116`|bsup-uncompressed|sup|1.39|1.62|0.11|
|`super`|`id.resp_h==52.85.83.116`|bsup-uncompressed|json|1.57|1.78|0.12|
|`super`|`id.resp_h==52.85.83.116`|sup|zeek|169.93|192.80|5.16|
|`super`|`id.resp_h==52.85.83.116`|sup|bsup|168.84|191.15|5.09|
|`super`|`id.resp_h==52.85.83.116`|sup|bsup-uncompressed|172.81|194.61|4.91|
|`super`|`id.resp_h==52.85.83.116`|sup|sup|167.40|187.45|4.16|
|`super`|`id.resp_h==52.85.83.116`|sup|json|167.00|187.98|4.67|
|`super`|`id.resp_h==52.85.83.116`|json|zeek|33.41|79.47|4.92|
|`super`|`id.resp_h==52.85.83.116`|json|bsup|35.15|81.74|5.48|
|`super`|`id.resp_h==52.85.83.116`|json|bsup-uncompressed|34.40|80.35|5.17|
|`super`|`id.resp_h==52.85.83.116`|json|sup|32.92|78.71|5.01|
|`super`|`id.resp_h==52.85.83.116`|json|json|33.68|79.77|5.23|
|`jq`|`-c '. \| select(.["id.resp_h"]=="52.85.83.116")'`|json|json|18.43|21.13|1.35|
