package transcode

import (
	"fmt"
	"github.com/bcc-code/bcc-media-flows/services/ffmpeg"
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
