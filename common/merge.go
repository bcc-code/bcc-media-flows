package common

type MergeInputItem struct {
	Path    string
	Start   float64
	End     float64
	Streams []int
}

type MergeInput struct {
	Title     string
	Items     []MergeInputItem
	OutputDir string
	WorkDir   string
	Duration  float64
}

type MergeResult struct {
	Path string
}
