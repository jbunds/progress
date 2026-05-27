#!/bin/bash

go test -tags integration -c -o integration_test .

# see also:
#
#   man time
#   man -S2 getrusage

base_cmd=(/usr/bin/time -l -h ./integration_test -test.run=TestStreamingStress --)

args=(
  # weigh-based accumulation API mode
  '-totaltasks  100 -loopiterations 100' # base case:                     smoke test
  '-totaltasks 1000 -loopiterations 1e7' # dense weight-based contention: heavy CAS race condition tracking on a fixed budget
  '-totaltasks  1e6 -loopiterations 1e6' # macro scale weight-based:      checks performance and layout stability at 1:1 ratio
  # fractional path allocation API mode
  '-totaltasks    0 -loopiterations 100' # small-scale micro-fractional mode:   exercises budget discovery with minimal iterations
  '-totaltasks    0 -loopiterations 1e6' # high-frequency fractional discovery: contention on p.AddTotal and p.Report loops
  '-totaltasks    0 -loopiterations 1e7' # high-load stress test:               checks memory stabilization and heap escapes over time
)

for a in "${args[@]}"; do
  echo "args: $a"
  { "${base_cmd[@]}" $a ; } 2>&1 | \
    awk '/real/ {
      print "time:\t", $1
    };
    /maximum resident set size/ {
      printf "rss:\t %.2f MB\n",   $1 / (1024 * 1024)
    };
    /peak memory footprint/ {
      printf "mem:\t %.2f MB\n\n", $1 / (1024 * 1024)
    }'
done
