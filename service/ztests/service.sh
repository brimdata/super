#!/bin/bash

function awaitfile {
  file=$1
  i=0
  until [ -f $file ]; do
    let i+=1
    if [ $i -gt 50 ]; then
      echo "db serve log:"
      cat db.log
      exit 1
    fi
    sleep 0.1
  done
}

db_root=$1
if [ -z "$db_root" ]; then
  db_root=db_root
fi
mkdir -p $db_root

portdir=$(mktemp -d)

super db serve -l=localhost:0 -db=$db_root -portfile=$portdir/db -log.level=warn $DB_EXTRA_FLAGS &> db.log &
db_pid=$!
awaitfile $portdir/db

trap "rm -rf $portdir; kill $db_pid;" EXIT

export SUPER_DB=http://localhost:$(cat $portdir/db)
export DB_PATH=$db_root
