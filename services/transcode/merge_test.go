package transcode

import (
	"github.com/bcc-code/bcc-media-flows/common"
	"github.com/bcc-code/bcc-media-flows/paths"

	"testing"
)

const TestRoot = "/Users/fredrikvedvik/Desktop/Transcoding/test/"

func Test_Merge(t *testing.T) {
	_, _ = MergeVideo(common.MergeInput{
		Title:     "Test",
		OutputDir: paths.MustParse("/Users/*/Desktop/Transcoding/test/"),
		Items: []common.MergeInputItem{
			{
				Start: 8,
				End:   12,
				Path:  paths.MustParse("/Users/*/Desktop/Transcoding/test/*.mp4"),
			},
			{
				Start: 10,
				End:   15,
				Path:  paths.MustParse("/Users/*/Desktop/Transcoding/test/*.mp4"),
			},
		},
	}, nil)
}

func Test_MergeAudio(t *testing.T) {
	_, _ = MergeAudio(common.MergeInput{
		Title:     "Test",
		OutputDir: paths.MustParse("/Users/*/Desktop/Transcoding/test/"),
		Items: []common.MergeInputItem{
			{
				Start: 0,
				End:   2,
				Streams: []int{
					1,
					2,
				},
				Path: paths.MustParse("/Users/*/Desktop/Transcoding/test/vi.mxf"),
			},
			{
				Start: 2,
				End:   4,
				Streams: []int{
					2,
					3,
				},
				Path: paths.MustParse("/Users/*/Desktop/Transcoding/test/vi.mxf"),
			},
			{
				Start: 2,
				End:   4,
				Streams: []int{
					0,
				},
				Path: paths.MustParse("/Users/*/Desktop/Transcoding/test/2.mp4"),
			},
		},
	}, nil)
}
