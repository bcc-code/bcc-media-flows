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
	Priority    rclone.Priority
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
	Priority    rclone.Priority
}

func (ua UtilActivities) RcloneMoveFile(ctx context.Context, input RcloneFileInput) (int, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Starting RcloneMoveFile")

	srcFs, srcRemote := input.Source.RcloneFsRemote()
	dstFs, dstRemote := input.Destination.RcloneFsRemote()

	res, err := rclone.MoveFile(
		srcFs, srcRemote,
		dstFs, dstRemote,
		input.Priority,
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
		input.Priority,
	)
	if err != nil {
		return 0, err
	}

	return res.JobID, nil
}

type RcloneSingleFileInput struct {
	File paths.Path
}

func (ua UtilActivities) RcloneCheckFileExists(ctx context.Context, input RcloneSingleFileInput) (bool, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Starting RcloneCheckFileExists")

	fs, remote := input.File.RcloneFsRemote()

	stats, err := rclone.Stat(fs, remote)
	if err != nil {
		return false, err
	}

	return stats != nil, nil
}
