#!/bin/bash

themes=($(egrep '^\s+name:\s+"[^"]+",' themes.go | # brittle hack...
	tr -d \" | tr -d , |
	awk '{ print $2 }'))

for theme in ${themes[@]}; do
	echo $theme
	go run -tags examples ./examples/smoke -forcetty -persistbar -theme $theme
	echo
done
