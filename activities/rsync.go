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

type RsyncIncrementalCopyResult struct {
	Size int64
}

func (l LiveActivities) RsyncIncrementalCopy(ctx context.Context, input RsyncIncrementalCopyInput) (*RsyncIncrementalCopyResult, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Starting RsyncIncrementalCopy")

	stopChan, cb := newHeartBeater[rsync.FileInfo](ctx)
	defer close(stopChan)

	err := rsync.IncrementalCopy(input.In, input.Out, cb)
	if err != nil {
		return nil, err
	}

	// Stat the destination file to get its size
	fileInfo, statErr := input.Out.Stat()
	if statErr != nil {
		logger.Error("Failed to stat destination file after copy", "error", statErr)
		return &RsyncIncrementalCopyResult{Size: 0}, nil
	}

	return &RsyncIncrementalCopyResult{Size: fileInfo.Size()}, nil
}
