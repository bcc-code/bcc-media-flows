package activities

import (
	"context"
	"fmt"
	"os"

	"github.com/bcc-code/bcc-media-flows/paths"
	"github.com/bcc-code/bcc-media-flows/services/ffmpeg"
	"go.temporal.io/sdk/activity"
)

type DemuxCheckParams struct {
	FilePath paths.Path
}

// DemuxCheck reads and decodes the whole file through ffmpeg, writing nothing,
// and returns every warning and error the demuxers, parsers and decoders logged. A broken
// file is a result, not a failure: the activity errors only when the check
// itself could not run.
func (va VideoActivities) DemuxCheck(ctx context.Context, params DemuxCheckParams) (*ffmpeg.DemuxCheckResult, error) {
	log := activity.GetLogger(ctx)
	activity.RecordHeartbeat(ctx, "DemuxCheck")
	log.Info("Starting DemuxCheck", "path", params.FilePath.Local())

	// ffmpeg reports a missing file as an ordinary error and exits, which would
	// turn a mount that has not caught up yet into a FAILED verdict. Stat first
	// so that case fails the attempt and Temporal retries it.
	if _, err := os.Stat(params.FilePath.Local()); err != nil {
		return nil, fmt.Errorf("demux check input: %w", err)
	}

	stop, progressCallback := registerProgressCallback(ctx)
	defer close(stop)

	result, err := ffmpeg.DemuxCheck(params.FilePath.Local(), nil, progressCallback)
	if err != nil {
		return nil, err
	}

	log.Info("DemuxCheck finished", "path", params.FilePath.Local(), "errors", result.Errors, "warnings", result.Warnings, "shortRead", result.ShortRead, "exitError", result.ExitError)
	return &result, nil
}
