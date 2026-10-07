module github.com/quietpelican42/graphemebreak/bench

go 1.26.0

require (
	github.com/quietpelican42/graphemebreak v0.0.0
	github.com/clipperhouse/uax29 v1.16.0
	github.com/rivo/uniseg v0.4.7
)

require golang.org/x/text v0.16.0 // indirect

replace github.com/quietpelican42/graphemebreak => ../
