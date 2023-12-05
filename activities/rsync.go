package activities

import (
	"context"

	"github.com/bcc-code/bccm-flows/paths"
	"github.com/bcc-code/bccm-flows/services/rsync"
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

	stopChan, _ := newHeartBeater[string](ctx)
	defer close(stopChan)

	err := rsync.IncrementalCopy(input.In, input.Out)
	if err != nil {
		return nil, err
	}

	return &RsyncIncrementalCopyResult{}, nil
}
