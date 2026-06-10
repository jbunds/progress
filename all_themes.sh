#!/bin/bash

themes=($(egrep '^\s+name:\s+"[^"]+",' themes.go |  # brittle hack, but it works...
	tr -d \" | tr -d , |
	awk '{ print $2 }'))

trap 'exit $((128 +  1))' SIGHUP  # SIGHUP  ==  1
trap 'exit $((128 +  2))' SIGINT  # SIGINT  ==  2
trap 'exit $((128 +  3))' SIGQUIT # SIGQUIT ==  3
trap 'exit $((128 + 15))' SIGTERM # SIGQUIT == 15

# the `go run` wrapper traps and handles SIGHUP, SIGINT, SIGQUIT, and SIGTERM
# signals, and prints an error message to stderr when it receives any of those
# signals (e.g., "exit status 130" when it receives a SIGINT signal), so the
# main Go program is compiled and then executed directly to avoid races between
# the `go run` wrapper and the main Go program both writing to stderr, which
# results in garbled output (e.g., "exit status 130t canceled)" caused by the
# `go run` wrapper writing "exit status 130" atop "stopped (context canceled)"
# written by the main Go program

go build -tags examples ./examples/smoke

# -forcetty:     render colored output when piped or redirected
# -persistbar:   don't clear the progress bar on exit
# -theme $theme: the color scheme used to render the progress bar

for theme in ${themes[@]}; do
	echo $theme
	./smoke -forcetty -persistbar -theme $theme
	status=$?
	if [[ $status -eq 129 ||
		  $status -eq 130 ||
		  $status -eq 131 ||
		  $status -eq 143 ]]; then
		exit $status
	fi
	echo
done
