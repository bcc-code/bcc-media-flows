package transcode

import (
	"fmt"
	"github.com/bcc-code/bccm-flows/common"
	"github.com/bcc-code/bccm-flows/services/ffmpeg"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

const root = "/Users/fredrikvedvik/Desktop/Transcoding/sotm7/"

func printProgress() (ffmpeg.ProgressCallback, chan struct{}) {
	var progress ffmpeg.Progress

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

	return func(p ffmpeg.Progress) {
		progress = p
	}, stopChan
}

func Test_MuxVideo(t *testing.T) {
	printer, stop := printProgress()
	defer close(stop)
	_, err := Mux(common.MuxInput{
		DestinationPath: "/Users/fredrikvedvik/Desktop/Transcoding/test/",
		VideoFilePath:   root + "SOTM_7v2123_SEQ.mxf",
		AudioFilePaths: map[string]string{
			"nor": root + "SOTM_7v2123_SEQ-nor.wav",
			"eng": root + "SOTM_7v2123_SEQ-eng.wav",
			"nld": root + "SOTM_7v2123_SEQ-nld.wav",
		},
	}, printer)

	assert.Nil(t, err)
}
