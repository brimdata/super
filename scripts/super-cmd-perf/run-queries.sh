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
  storage="/mnt/"
else
  storage=""
fi

warmups=1
runs=1
report="$rundir/report_$(basename "$rundir").md"
csv_report="$rundir/report_$(basename "$rundir").csv"

function run_query {
  cmd="$1"
  shift
  queryfile="$1"
  shift
  source="$1"
  shift
  outputfile="$rundir/$cmd-$queryfile-$source.out"

  final_query=$(mktemp)

  DUCKDB_MEMORY_LIMIT="${DUCKDB_MEMORY_LIMIT:-}"
  if [ "$cmd" == "duckdb" ] && [ -n "$DUCKDB_MEMORY_LIMIT" ]; then
    echo 'SET memory_limit = '\'"${DUCKDB_MEMORY_LIMIT}"\''; ' >> "$final_query"
  fi

  if [ "$source" == "gha" ]; then
    sed -e "s/__SOURCE__/$source/" "queries/$queryfile" >> "$final_query"
  else
    sed -e "s/__SOURCE__/${storage//\//\\/}${source}/" "queries/$queryfile" >> "$final_query"
  fi

  if [ "$cmd" == "super" ]; then
    if [ "$source" == "gha.parquet" ] || [ "$source" == "gha.csup" ]; then
      cmd="SUPER_VAM=1 super"
    fi
    cmd="$cmd -s -I $final_query"
  elif [ "$cmd" == "duckdb" ]; then
    if [ "$source" == "gha" ]; then
      cmd="duckdb ${storage}gha.db"
    fi
    cmd="$cmd < $final_query"
  elif [ "$cmd" == "datafusion" ]; then
    cmd="datafusion-cli --file $final_query"
  elif [[ "$cmd" == "clickhouse"* ]]; then
    cmd="$cmd --queries-file $final_query"
  fi

  echo -e "About to execute\n================\n$cmd\n\nWith query\n==========" > "$outputfile"
  cat "$final_query" >> "$outputfile"
  echo >> "$outputfile"

  { hyperfine \
      --show-output \
      --warmup $warmups \
      --runs $runs \
      --time-unit second \
      "$cmd" ;
  } \
    >> "$outputfile" \
    2>&1

  rm -f "$final_query"
}

echo "|**Tool**|**Format**|**search**|**search+**|**count**|**agg**|**union**|" >> "$report"
echo "|-|-|-|-|-|-|-|" >> "$report"
echo "Tool,Format,search,search+,count,agg,union" > "$csv_report"

# Skipping CSUP for now due to https://github.com/brimdata/super/issues/5550
#
for source in gha.bsup gha.parquet # gha.csup
do
  echo -n "|\`super\`|\`${source/gha./}\`|" >> "$report"
  echo -n "super,${source/gha./}" >> "$csv_report"
  for queryfile in search.spq search+.spq count.sql agg.sql union.spq
  do
    run_query super $queryfile "$source"
    result=$(grep Time < "$rundir/super-$queryfile-$source.out" | awk '{ print $4 }')
    echo -n "$result" >> "$report"
    echo -n "|" >> "$report"
    echo -n ",$result" >> "$csv_report"
  done
  echo >> "$report"
  echo >> "$csv_report"
done

for source in gha gha.parquet
do
  duckdb_source=${source/gha\./}
  duckdb_source=${duckdb_source/gha/db}
  echo -n "|\`duckdb\`|\`$duckdb_source\`|" >> "$report"
  echo -n "duckdb,$duckdb_source" >> "$csv_report"
  for queryfile in search.sql search+.sql count.sql agg.sql union.sql
  do
    run_query duckdb $queryfile "$source"
    result=$(grep Time < "$rundir/duckdb-$queryfile-$source.out" | awk '{ print $4 }')
    echo -n "$result" >> "$report"
    echo -n "|" >> "$report"
    echo -n ",$result" >> "$csv_report"
  done
  echo >> "$report"
  echo >> "$csv_report"
done

echo -n "|\`datafusion\`|\`parquet\`|" >> "$report"
echo -n "datafusion,parquet" >> "$csv_report"
for queryfile in search.sql search+.sql count.sql agg.sql union-datafusion.sql
do
  run_query datafusion $queryfile gha.parquet
  result=$(grep Time < "$rundir/datafusion-$queryfile-$source.out" | awk '{ print $4 }')
  echo -n "$result" >> "$report"
  echo -n "|" >> "$report"
  echo -n ",$result" >> "$csv_report"
done
echo >> "$report"
echo >> "$csv_report"

echo -n "|\`clickhouse\`|\`parquet\`|" >> "$report"
echo -n "clickhouse,parquet" >> "$csv_report"
for queryfile in search.sql search+.sql count.sql agg.sql union-clickhouse.sql
do
  run_query clickhouse $queryfile gha.parquet
  result=$(grep Time < "$rundir/clickhouse-$queryfile-$source.out" | awk '{ print $4 }')
  echo -n "$result" >> "$report"
  echo -n "|" >> "$report"
  echo -n ",$result" >> "$csv_report"
done
echo >> "$report"
echo >> "$csv_report"

if [ -n "$RUNNING_ON_AWS_EC2" ]; then
  sudo systemctl start clickhouse-server
  echo -n "|\`clickhouse\`|\`db\`|" >> "$report"
  echo -n "clickhouse,db" >> "$csv_report"
  for queryfile in search-clickhouse-db.sql search+-clickhouse-db.sql count-clickhouse-db.sql agg-clickhouse-db.sql union-clickhouse-db.sql
  do
    if [ "$queryfile" == "union-clickhouse-db.sql" ]; then
      echo -n "N/A|" >> "$report"
      echo -n ",N/A" >> "$csv_report"
      continue
    fi
    run_query clickhouse-client $queryfile gha
    result=$(grep Time < "$rundir/clickhouse-client-$queryfile-$source.out" | awk '{ print $4 }')
    echo -n "$result" >> "$report"
    echo -n "|" >> "$report"
    echo -n ",$result" >> "$csv_report"
  done
  sudo systemctl stop clickhouse-server
  echo >> "$report"
  echo >> "$csv_report"
fi
