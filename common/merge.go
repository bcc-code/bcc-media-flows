package common

import (
	"github.com/bcc-code/bcc-media-flows/paths"
	"github.com/bcc-code/bcc-media-flows/services/vidispine"
)

type MergeInputItem struct {
	Path        paths.Path
	Start       float64
	End         float64
	StartOffset float64
	Streams     []vidispine.AudioStream
}

type MergeInput struct {
	Title     string
	Items     []MergeInputItem
	OutputDir paths.Path
	WorkDir   paths.Path
	Duration  float64
}

type MergeResult struct {
	Path paths.Path
}
