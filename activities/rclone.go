package activities

import (
	"context"
	"fmt"
	"time"

	"github.com/bcc-code/bcc-media-flows/paths"

	"github.com/bcc-code/bcc-media-flows/services/rclone"
	"go.temporal.io/sdk/activity"
)

func (ua UtilActivities) RcloneWaitForJob(ctx context.Context, jobID int) (bool, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Starting RcloneWaitForJob")

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

func (ua UtilActivities) RcloneCopyDir(ctx context.Context, input RcloneCopyDirInput) (int, error) {
	activity.RecordHeartbeat(ctx, "Rclone CopyDir")
	activity.GetLogger(ctx).Debug(fmt.Sprintf("Rclone CopyDir: %s -> %s", input.Source, input.Destination))

	res, err := rclone.CopyDir(input.Source, input.Destination)
	if err != nil {
		return 0, err
	}
	return res.JobID, nil
}

type RcloneFileInput struct {
	Source      paths.Path
	Destination paths.Path
}

func (ua UtilActivities) RcloneMoveFile(ctx context.Context, input RcloneFileInput) (int, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Starting RcloneMoveFile")

	srcFs, srcRemote := input.Source.RcloneFsRemote()
	dstFs, dstRemote := input.Destination.RcloneFsRemote()

	res, err := rclone.MoveFile(
		srcFs, srcRemote,
		dstFs, dstRemote,
	)
	if err != nil {
		return 0, err
	}

	return res.JobID, nil
}

func (ua UtilActivities) RcloneCopyFile(ctx context.Context, input RcloneFileInput) (int, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Starting RcloneCopyFile")

	srcFs, srcRemote := input.Source.RcloneFsRemote()
	dstFs, dstRemote := input.Destination.RcloneFsRemote()

	res, err := rclone.CopyFile(
		srcFs, srcRemote,
		dstFs, dstRemote,
	)
	if err != nil {
		return 0, err
	}

	return res.JobID, nil
}
