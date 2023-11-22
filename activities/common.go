package activities

import (
	"context"
	"time"

	"github.com/bcc-code/bccm-flows/services/ffmpeg"
	"go.temporal.io/sdk/activity"
)

func registerProgressCallback(ctx context.Context) (chan struct{}, func(ffmpeg.Progress)) {
	return newHeartBeater[ffmpeg.Progress](ctx)
}

func newHeartBeater[T any](ctx context.Context) (chan struct{}, func(T)) {
	var info T

	cb := func(i T) {
		info = i
	}

	stopChan := make(chan struct{})

	go func() {
		timer := time.NewTicker(time.Second * 15)
		defer timer.Stop()

		for {
			select {
			case <-timer.C:
				activity.RecordHeartbeat(ctx, info)
				if ctx.Err() != nil {
					return
				}
			case <-stopChan:
				return
			}
		}
	}()

	return stopChan, cb
}

func simpleHeartBeater(ctx context.Context) chan struct{} {
	stopChan := make(chan struct{})

	go func() {
		timer := time.NewTicker(time.Second * 15)
		defer timer.Stop()

		for {
			select {
			case <-timer.C:
				activity.RecordHeartbeat(ctx)
				if ctx.Err() != nil {
					return
				}
			case <-stopChan:
				return
			}
		}
	}()

	return stopChan
}
