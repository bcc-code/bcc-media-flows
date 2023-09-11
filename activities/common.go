package activities

import (
	"context"
	"github.com/bcc-code/bccm-flows/services/ffmpeg"
	"go.temporal.io/sdk/activity"
	"time"
)

func registerProgressCallback(ctx context.Context) (chan struct{}, func(ffmpeg.Progress)) {
	var current ffmpeg.Progress

	progressCallback := func(percent ffmpeg.Progress) {
		current = percent
	}

	stopChan := make(chan struct{})

	go func() {
		timer := time.NewTicker(time.Second * 15)
		defer timer.Stop()

		for {
			select {
			case <-timer.C:
				activity.RecordHeartbeat(ctx, current)
			case <-stopChan:
				return
			}
		}
	}()

	return stopChan, progressCallback
}
