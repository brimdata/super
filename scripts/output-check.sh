#!/bin/bash

# Zed is the only known tool set that outputs data in BSUP formats. Sample
# BSUP data from super is stored in the https://github.com/brimdata/zed-sample-data
# repo. Therefore, if a change in Zed causes the BSUP output format to change,
# we'll want to know about it ASAP, since if it's a bug we'll want to fix it
# in Zed, and if it's an intentional enhancement we'll want to update the BSUP
# files in zed-sample-data so users are always finding a current copy.
#
# This script automates this check by running the Zeek TSV logs from
# zed-sample-data through super, produces output in four BSUP variations, and
# checks that the MD5 hashes for the outputs still match the hashes stored
# in the zed-sample-data repo.

# We're intentionally not running with "set -eo pipefail" because we want to
# let all permutations run and allow the final error text to be seen before
# explicitly returning the intended error code.

cd zed-sample-data
scripts/check_md5sums.sh bsup
BSUP_SUCCESS="$?"
echo
scripts/check_md5sums.sh bsup-uncompressed
BSUP_UNCOMPRESSED_SUCCESS="$?"
echo
scripts/check_md5sums.sh sup
SUP_SUCCESS="$?"
echo

if (( BSUP_SUCCESS == 0 && BSUP_UNCOMPRESSED_SUCCESS == 0 && SUP_SUCCESS == 0)); then
  exit 0
else
  echo
  echo "------------------------------------------------------------------------------"
  echo "Output format has changed. If your work intentionally changed BSUP or SUP"
  echo "output and hence you do not suspect a bug, either update the zed-sample-data"
  echo "repo with new output files and MD5 hashes to make this test pass, or open a zed"
  echo "issue and include the output from this script and someone else will take care"
  echo "of it ASAP."
  exit 1
fi
