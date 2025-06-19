#!/bin/bash

function awaitfile {
  file=$1
  i=0
  until [ -f $file ]; do
    let i+=1
    if [ $i -gt 50 ]; then
      echo "lake serve log:"
      cat lake.log
      exit 1
    fi
    sleep 0.1
  done
}

lakeroot=$1
if [ -z "$lakeroot" ]; then
  lakeroot=lakeroot
fi
mkdir -p $lakeroot

portdir=$(mktemp -d)

super db serve -l=localhost:0 -db=$lakeroot -portfile=$portdir/lake -log.level=warn $LAKE_EXTRA_FLAGS &> lake.log &
lakepid=$!
awaitfile $portdir/lake

trap "rm -rf $portdir; kill $lakepid;" EXIT

export SUPER_DB=http://localhost:$(cat $portdir/lake)
export LAKE_PATH=$lakeroot
