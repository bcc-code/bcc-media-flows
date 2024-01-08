package activities

import (
	"context"

	"github.com/bcc-code/bcc-media-flows/paths"
	"github.com/bcc-code/bcc-media-flows/services/rsync"
	"go.temporal.io/sdk/activity"
)

type RsyncIncrementalCopyInput struct {
	In  paths.Path
	Out paths.Path
}

type RsyncIncrementalCopyResult struct{}

func RsyncIncrementalCopy(ctx context.Context, input RsyncIncrementalCopyInput) (*RsyncIncrementalCopyResult, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Starting RsyncIncrementalCopy")

	stopChan, cb := newHeartBeater[rsync.FileInfo](ctx)
	defer close(stopChan)

	err := rsync.IncrementalCopy(input.In, input.Out, cb)
	if err != nil {
		return nil, err
	}

	return &RsyncIncrementalCopyResult{}, nil
}
