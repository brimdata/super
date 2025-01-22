# Query Performance From `super` Command Doc

These scripts were used to generate the results in the
[Performance](https://zed.brimdata.io/docs/next/commands/super#performance)
section of the [`super` command doc](https://zed.brimdata.io/docs/next/commands/super).
The scripts have been made available to allow for easy reproduction of the
results under different conditions and/or as tested systems evolve.

# Environments

The scripts were written to be easily run in two different environments.

## AWS

As an environment that's available to everyone, the scripts were developed
primarily for use on a "scratch" EC2 instance in [AWS](https://aws.amazon.com/).
Specifically, we chose the [`m6idn.2xlarge`](https://aws.amazon.com/ec2/instance-types/m6i/)
instance that has the following specifications:

* 8x vCPU
* 32 GB of RAM
* 474 GB NVMe instance SSD

The instance SSD in particular was seen as important to ensure consistent I/O
performance.

Assuming a freshly-created `m6idn.2xlarge` instance running Ubuntu 24.04, to
start the run:

```
curl -s https://raw.githubusercontent.com/brimdata/super/refs/heads/main/scripts/super-cmd-perf/benchmark.sh | bash -xv 2>&1 | tee runlog.txt
```

The run proceeds in three phases:

1. **(AWS only)** Instance SSD is formatted and required tools & data platforms tools are downloaded/installed
2. Test data is downloaded and loaded into needed storage formats
3. Queries are executed on all data platforms

The scripts only run with ClickHouse's [beta JSON type](https://clickhouse.com/blog/a-new-powerful-json-data-type-for-clickhouse)
on AWS because when we attempted to load data to this type on our Macbooks
that have 16 GB of RAM it consistently failed with a "too many open files"
error.

As the benchmarks may take a long time to run, the use of [`screen`](https://en.wikipedia.org/wiki/GNU_Screen)
or a similar "detachable" terminal tool is recommended in case your remote
network connection drops during a run.

## macOS/other

Whereas on [AWS](#aws) the scripts assume they're in a "scratch" environment
where it may format the instance SSD for optimal storage and install required
software, on other systems such as macOS it's assumed the required data
platforms are already installed, and it will skip ahead right to
downloading/loading test data and then running queries.

For instance on macOS, the software needed can be first installed via:

```
brew install hyperfine datafusion duckdb clickhouse go
go install github.com/brimdata/super/cmd/super@main
```

Then clone the [super repo](https://github.com/brimdata/super.git) and run the
benchmarks.

```
git clone https://github.com/brimdata/super.git
cd scripts/super-cmd-perf
./benchmark.sh
```

All test data will remain in this directory.

# Results

Results from the run will accumulate in a subdirectory named for the date/time
when the run started, e.g., `2024-11-19_01:10:30/`. In this directory, summary
reports will be created in files ending in `.md` and `.csv` extensions, and
details from each individual step in generating the results will be in files
ending in `.out`. If run on AWS using the [`curl` command line shown above](#aws),
the `runlog.txt` will also be present that holds the full console output of the
entire run.

An archive of results from our most recent run of the benchmarks on November
26, 2024 can be downloaded [here](https://super-cmd-perf.s3.us-east-2.amazonaws.com/2024-11-26_03-17-25.tgz).

# Debugging

The scripts are configured to exit immediately if failures occur during the
run. If you encounter a failure, look in the results directory for the `.out`
file mentioned last in the console output as this will contain any detailed
error message from the operation that experienced the failure.

A problem that was encountered when developing the scripts that you may also
encounter is DuckDB running out of memory. Specifically, this happened when
we tried to run the scripts on an Intel-based Macbook with only 16 GB of
RAM, and this is part of why we used an AWS instance with 32 GB of RAM as the
reference platform. On the Macbooks, we found we could work around the memory
problem by telling DuckDB it had the use of more memory than its default
[80% heuristic for `memory_limit`](https://duckdb.org/docs/configuration/overview.html).
The scripts support an environment variable to make it easy to increase this
value, e.g., we found the scripts ran successfully at 16 GB:

```
$ DUCKDB_MEMORY_LIMIT="16GB" ./benchmark.sh
```

Of course, this ultimately caused swapping on our Macbook and a significant
hit to performance, but it at least allowed the scripts to run without
failure.
