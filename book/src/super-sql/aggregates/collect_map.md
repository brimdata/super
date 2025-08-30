### Aggregate Function

&emsp; **collect_map** &mdash; aggregate map values into a single map

### Synopsis
```
collect_map(|{any:any}|) -> |{any:any}|
```

### Description

The _collect_map_ aggregate function combines map inputs into a single map output.
If _collect_map_ receives multiple values for the same key, the last value received is
retained. If the input keys or values vary in type, the return type will be a map
of union of those types.

### Examples

Combine a sequence of records into a map:
```mdtest-spq
# spq
collect_map(|{stock:price}|)
# input
{stock:"APPL",price:145.03}
{stock:"GOOG",price:87.07}
# expected output
|{"APPL":145.03,"GOOG":87.07}|
```

Continuous collection over a simple sequence:
```mdtest-spq
# spq
values collect_map(this)
# input
|{"APPL":145.03}|
|{"GOOG":87.07}|
|{"APPL":150.13}|
# expected output
|{"APPL":145.03}|
|{"APPL":145.03,"GOOG":87.07}|
|{"APPL":150.13,"GOOG":87.07}|
```

Create maps by key:
```mdtest-spq {data-layout="stacked"}
# spq
collect_map(|{stock:price}|) by day | sort
# input
{stock:"APPL",price:145.03,day:0}
{stock:"GOOG",price:87.07,day:0}
{stock:"APPL",price:150.13,day:1}
{stock:"GOOG",price:89.15,day:1}
# expected output
{day:0,collect_map:|{"APPL":145.03,"GOOG":87.07}|}
{day:1,collect_map:|{"APPL":150.13,"GOOG":89.15}|}
```
