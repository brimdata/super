#!/bin/bash -xv
set -euo pipefail
pushd "$(cd "$(dirname "$0")" && pwd)"

if [ "$#" -ne 1 ]; then
  echo "Specify results directory string"
  exit 1
fi
rundir="$(pwd)/$1"
mkdir -p "$rundir"

RUNNING_ON_AWS_EC2="${RUNNING_ON_AWS_EC2:-}"
if [ -n "$RUNNING_ON_AWS_EC2" ]; then
  cp clickhouse-table-create.sql /mnt
  cd /mnt
fi

function run_cmd {
  outputfile="$1"
  shift
  { hyperfine \
      --show-output \
      --warmup 0 \
      --runs 1 \
      --time-unit second \
      "$@" ;
  } \
    > "$outputfile" \
    2>&1
}

mkdir gharchive_gz
cd gharchive_gz
for num in $(seq 0 23)
do
  curl -L -O "https://data.gharchive.org/2023-02-08-${num}.json.gz"
done
cd ..

DUCKDB_MEMORY_LIMIT="${DUCKDB_MEMORY_LIMIT:-}"
if [ -n "$DUCKDB_MEMORY_LIMIT" ]; then
  increase_duckdb_memory_limit='SET memory_limit = '\'"${DUCKDB_MEMORY_LIMIT}"\''; '
else
  increase_duckdb_memory_limit=""
fi

run_cmd \
  "$rundir/duckdb-table-create.out" \
  "duckdb gha.db -c \"${increase_duckdb_memory_limit}CREATE TABLE gha AS FROM read_json('gharchive_gz/*.json.gz', union_by_name=true)\""

run_cmd \
  "$rundir/duckdb-parquet-create.out" \
  "duckdb gha.db -c \"${increase_duckdb_memory_limit}COPY (from gha) TO 'gha.parquet'\""

run_cmd \
  "$rundir/super-bsup-create.out" \
  "super -o gha.bsup gharchive_gz/*.json.gz"

run_cmd \
  "$rundir/super-csup-create.out" \
  "super -f csup -o gha.csup gharchive_gz/*.json.gz"

if [ -n "$RUNNING_ON_AWS_EC2" ]; then
  sudo mkdir -p /var/lib/clickhouse/user_files
  sudo chown clickhouse:clickhouse /var/lib/clickhouse/user_files
  sudo ln -s /mnt/gharchive_gz /var/lib/clickhouse/user_files/gharchive_gz
  sudo systemctl start clickhouse-server
  sleep 5
  run_cmd \
    "$rundir/clickhouse-table-create.out" \
    "clickhouse-client < clickhouse-table-create.sql"
  sudo systemctl stop clickhouse-server
  sudo du -sh clickhouse/store
fi

du -h gha.db gha.parquet gha.bsup gharchive_gz
