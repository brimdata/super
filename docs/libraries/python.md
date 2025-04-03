---
weight: 3
title: Python
---

Zed includes preliminary support for Python-based interaction
with a Zed lake.
The Zed Python package supports loading data into a Zed lake as well as
querying and retrieving results in the [ZJSON format](../formats/zjson.md).
The Python client interacts with the Zed lake via the REST API served by
[`super db serve`](../commands/super-db.md#serve).

This approach works adequately when high data throughput is not required.
We plan to introduce native [Super Binary](../formats/bsup.md) support for
Python that should increase performance substantially for more
data intensive workloads.

## Installation

Install the latest version like this:
```sh
pip3 install "git+https://github.com/brimdata/super#subdirectory=python/zed"
```

Install the version compatible with a local `zed` like this:
```sh
pip3 install "git+https://github.com/brimdata/super@$(zed -version | cut -d ' ' -f 2)#subdirectory=python/zed"
```

## Example

To run this example, first start a Zed lake service from your shell:
```sh
zed init -lake scratch
zed serve -lake scratch
```
> Or you can launch the [Zui app](https://zui.brimdata.io) and it will run a Zed lake service
> on the default port at `http://localhost:9867`.

Then, in another shell, use Python to create a pool, load some data,
and run a query:
```sh
python3 <<EOF
import zed

# Connect to the default lake at http://localhost:9867.  To use a
# different lake, supply its URL via the ZED_LAKE environment variable
# or as an argument here.
client = zed.Client()

client.create_pool('TestPool')

# Load some SUP records from a string.  A file-like object also works.
# Data format is detected automatically and can be CSV, JSON, SUP, Zeek TSV,
# ZJSON, or ZNG.
.client.load('TestPool', '{s:"hello"} {s:"world"}')

# Begin executing a Zed query for all values in TestPool.
# This returns an iterator, not a container.
values = client.query('from TestPool')

# Stream values from the server.
for val in values:
    print(val)
EOF
```

You should see this output:
```
{'s': 'world'}
{'s': 'hello'}
```
