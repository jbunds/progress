#!/bin/bash

trap 'exit $((128 +  1))' SIGHUP  # SIGHUP  ==  1
trap 'exit $((128 +  2))' SIGINT  # SIGINT  ==  2
trap 'exit $((128 +  3))' SIGQUIT # SIGQUIT ==  3
trap 'exit $((128 + 15))' SIGTERM # SIGQUIT == 15

echo 'executing TestRenderLoop_MemoryAllocRegression... (system-dependent ETA: ~3 seconds)'
echo

go test -tags integration -v -run TestRenderLoop_MemoryAllocRegression

echo

echo 'executing TestStreamingStress with various parameters... (system-dependent ETA: ~3 minutes)'
echo

go test -tags integration -c -o integration_test .

# see also:
#
#   man time
#   man -S2 getrusage

base_cmd=(/usr/bin/time -l -h ./integration_test -test.run=TestStreamingStress --)

args=(
  # weigh-based accumulation API mode
  '-totaltasks  100 -loopiterations 100' # base case:                     smoke test
  '-totaltasks 1000 -loopiterations 1e8' # dense weight-based contention: heavy CAS race condition tracking on a fixed budget
  '-totaltasks  1e6 -loopiterations 1e6' # macro scale weight-based:      checks performance and layout stability at 1:1 ratio
  # fractional path allocation API mode
  '-totaltasks    0 -loopiterations 100' # small-scale micro-fractional mode:   exercises budget discovery with minimal iterations
  '-totaltasks    0 -loopiterations 1e6' # high-frequency fractional discovery: contention on p.AddTotal and p.Report loops
  '-totaltasks    0 -loopiterations 1e8' # high-load stress test:               checks memory stabilization and heap escapes over time
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
	status=$?
	if [[ $status -eq 129 ||
	      $status -eq 130 ||
	      $status -eq 131 ||
	      $status -eq 143 ]]; then
		exit $status
	fi
done
