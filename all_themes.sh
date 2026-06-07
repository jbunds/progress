#!/bin/bash

themes=($(egrep '^\s+name:\s+"[^"]+",' themes.go | tr -d \" | tr -d , | awk '{ print $2 }'))  # brittle hack...

for theme in ${themes[@]}; do
	echo $theme
	go run -tags examples ./examples/smoke -theme $theme -forcetty -persistbar
	echo
done
