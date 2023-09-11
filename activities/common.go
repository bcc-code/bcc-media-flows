package activities

import (
	"context"
	"github.com/bcc-code/bccm-flows/services/transcode"
	"go.temporal.io/sdk/activity"
	"time"
)

func registerProgressCallback(ctx context.Context) (chan struct{}, func(transcode.Progress)) {
	var current transcode.Progress

	progressCallback := func(percent transcode.Progress) {
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
