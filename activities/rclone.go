package activities

import (
	"context"
	"fmt"
	"github.com/bcc-code/bccm-flows/paths"
	"time"

	"github.com/bcc-code/bccm-flows/services/rclone"
	"go.temporal.io/sdk/activity"
)

func waitForJob(ctx context.Context, jobID int) (bool, error) {
	for {
		job, err := rclone.CheckJobStatus(jobID)
		if err != nil {
			return false, err
		}
		activity.RecordHeartbeat(ctx, job)
		if job == nil {
			return false, nil
		}
		if job.Finished {
			if !job.Success {
				return false, fmt.Errorf("rclone job failed: %s", job.Error)
			}
			return true, nil
		}
		time.Sleep(time.Second * 10)
	}
}

type RcloneCopyDirInput struct {
	Source      string
	Destination string
}

func RcloneCopyDir(ctx context.Context, input RcloneCopyDirInput) (bool, error) {
	activity.RecordHeartbeat(ctx, "Rclone CopyDir")

	res, err := rclone.CopyDir(input.Source, input.Destination)
	if err != nil {
		return false, err
	}

	return waitForJob(ctx, res.JobID)
}

type RcloneMoveFileInput struct {
	Source      paths.Path
	Destination paths.Path
}

func RcloneMoveFile(ctx context.Context, input RcloneMoveFileInput) (bool, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Starting RcloneMoveFile")

	srcFs, srcRemote := input.Source.RcloneFsRemote()
	dstFs, dstRemote := input.Destination.RcloneFsRemote()

	res, err := rclone.MoveFile(
		srcFs, srcRemote,
		dstFs, dstRemote,
	)
	if err != nil {
		return false, err
	}

	return waitForJob(ctx, res.JobID)
}
