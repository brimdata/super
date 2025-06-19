#!/bin/bash

# This file simulates a running version of Zui desktop. It forks a service process
# then sits forever on the main thread.

function awaitdeadservice {
  i=0
  function servicealive { kill -0 $LAKE_PID 2> /dev/null; }
  while servicealive ; do
    let i+=1
    if [ $i -gt 50 ]; then
      echo "timed out waiting for service to exit" 
      exit 1
    fi
    sleep 0.1
  done
}

function awaitfile {
  file=$1
  i=0
  until [ -f $file ]; do
    let i+=1
    if [ $i -gt 50 ]; then
      echo "timed out waiting for file \"$file\" to appear"
      exit 1
    fi
    sleep 0.1
  done
}

mkdir -p lakeroot
lakeroot=lakeroot
tempdir=$(mktemp -d)

mockzui -lake="$lakeroot" -portfile="$tempdir/port" -pidfile="$tempdir/pid" &
mockzuipid=$!

# wait for service to start
awaitfile $tempdir/port
awaitfile $tempdir/pid

export SUPER_DB=http://localhost:$(cat $tempdir/port)
export LAKE_PID=$(cat $tempdir/pid)
export MOCKZUI_PID=$mockzuipid

# ensure that lake service process isn't leaked
trap "kill -9 $LAKE_PID 2>/dev/null || :" EXIT
rm -rf $tempdir
