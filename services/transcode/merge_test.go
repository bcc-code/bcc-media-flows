package transcode

import (
	"fmt"
	"github.com/bcc-code/bccm-flows/common"
	"github.com/stretchr/testify/assert"
	"testing"
)

const TestRoot = "/Users/fredrikvedvik/Desktop/Transcoding/test/"

func Test_Merge(t *testing.T) {
	_, _ = MergeVideo(common.MergeInput{
		Title:     "Test",
		OutputDir: "/Users/*/Desktop/Transcoding/test/",
		Items: []common.MergeInputItem{
			{
				Start: 8,
				End:   12,
				Path:  "/Users/*/Desktop/Transcoding/test/*.mp4",
			},
			{
				Start: 10,
				End:   15,
				Path:  "/Users/*/Desktop/Transcoding/test/*.mp4",
			},
		},
	}, nil)
}

func Test_MergeAudio(t *testing.T) {
	_, _ = MergeAudio(common.MergeInput{
		Title:     "Test",
		OutputDir: "/Users/*/Desktop/Transcoding/test/",
		Items: []common.MergeInputItem{
			{
				Start: 0,
				End:   2,
				Streams: []int{
					1,
					2,
				},
				Path: "/Users/*/Desktop/Transcoding/test/vi.mxf",
			},
			{
				Start: 2,
				End:   4,
				Streams: []int{
					2,
					3,
				},
				Path: "/Users/*/Desktop/Transcoding/test/vi.mxf",
			},
			{
				Start: 2,
				End:   4,
				Streams: []int{
					0,
				},
				Path: "/Users/*/Desktop/Transcoding/test/2.mp4",
			},
		},
	}, nil)
}

func Test_MergeSubtitles(t *testing.T) {
	res, err := MergeSubtitles(common.MergeInput{
		Title:     "Test",
		OutputDir: TestRoot,
		WorkDir:   TestRoot + "tmp/",
		Items: []common.MergeInputItem{
			{
				Start: 10,
				End:   30,
				Path:  TestRoot + "1.srt",
			},
			{
				Start: 50,
				End:   80,
				Path:  TestRoot + "1.srt",
			},
		},
	})

	fmt.Println(res)

	assert.Nil(t, err)
}
