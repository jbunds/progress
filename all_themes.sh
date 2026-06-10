#!/bin/bash

themes=($(egrep '^\s+name:\s+"[^"]+",' themes.go |  # brittle hack, but it works...
	tr -d \" | tr -d , |
	awk '{ print $2 }'))

# -forcetty:        render colored output when piped or redirected
# -persistbar:      don't clear the progress bar on exit
# -tracker percent: render minimal progress status text in the progress bar

for theme in ${themes[@]}; do
	echo $theme
	go run -tags examples ./examples/smoke \
	-forcetty                              \
	-persistbar                            \
	-theme $theme
	echo
done
