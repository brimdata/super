# Python

SuperDB includes preliminary support for Python-based interaction
with a SuperDB database.

The Python package supports loading data into a database as well as
querying and retrieving results in the [JSUP format](../../formats/jsup.md).
The Python client interacts with the database via the REST API served by
[`super db serve`](../../command/db-serve.md).

This approach works adequately when high data throughput is not required.
We plan to introduce native binary format support for
Python that should increase performance substantially for more
data intensive workloads.

## Installation

Install the latest version like this:
```sh
pip3 install "git+https://github.com/brimdata/super#subdirectory=python/superdb"
```

Install the version compatible with a particular version of SuperDB like this:
```sh
pip3 install "git+https://github.com/brimdata/super@$(super -version | cut -d ' ' -f 2)#subdirectory=python/superdb"
```

## Example

To run this example, first start a SuperDB service from your shell:
```sh
super db init -db scratch
super db serve -db scratch
```
> Or you can launch the [Desktop app](https://zui.brimdata.io) and it will run a
> SuperDB service on the default port at `http://localhost:9867`.

Then, in another shell, use Python to create a pool, load some data,
and run a query:
```sh
python3 <<EOF
import superdb

# Connect to the default lake at http://localhost:9867.  To use a
# different lake, supply its URL via the SUPER_DB environment variable
# or as an argument here.
client = superdb.Client()

client.create_pool('TestPool')

# Load some SUP records from a string.  A file-like object also works.
# Data format is detected automatically and can be BSUP, CSV, JSON, SUP,
# Zeek TSV, or JSUP.
client.load('TestPool', '{s:"hello"} {s:"world"}')

# Begin executing a SuperDB query for all values in TestPool.
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
