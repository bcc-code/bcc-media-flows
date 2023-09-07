package transcode

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

const root = "/Users/fredrikvedvik/Desktop/Transcoding/sotm7/"

func printProgress() (func(Progress), chan struct{}) {
	var progress Progress

	stopChan := make(chan struct{})

	go func() {
		timer := time.NewTicker(time.Second * 5)
		defer timer.Stop()

		for {
			select {
			case <-timer.C:
				fmt.Println(progress)
			case <-stopChan:
				return
			}
		}
	}()

	return func(p Progress) {
		progress = p
	}, stopChan
}

func Test_MuxVideo(t *testing.T) {
	printer, stop := printProgress()
	defer close(stop)
	_, err := Mux(MuxVideoInput{
		DestinationPath: "/Users/fredrikvedvik/Desktop/Transcoding/test/",
		Bitrate:         "5M",
		FrameRate:       25,
		Width:           1280,
		Height:          720,
		VideoFilePath:   root + "SOTM_7v2123_SEQ.mxf",
		AudioFilePaths: map[string]string{
			"nor": root + "SOTM_7v2123_SEQ-nor.wav",
			"eng": root + "SOTM_7v2123_SEQ-eng.wav",
			"nld": root + "SOTM_7v2123_SEQ-nld.wav",
		},
	}, printer)

	assert.Nil(t, err)
}
