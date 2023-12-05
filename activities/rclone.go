package activities

import (
	"context"
	"fmt"
	"time"

	"github.com/bcc-code/bccm-flows/paths"

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

type RcloneFileInput struct {
	Source      paths.Path
	Destination paths.Path
}

func RcloneMoveFile(ctx context.Context, input RcloneFileInput) (bool, error) {
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

func RcloneCopyFile(ctx context.Context, input RcloneFileInput) (bool, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Starting RcloneCopyFile")

	srcFs, srcRemote := input.Source.RcloneFsRemote()
	dstFs, dstRemote := input.Destination.RcloneFsRemote()

	res, err := rclone.CopyFile(
		srcFs, srcRemote,
		dstFs, dstRemote,
	)
	if err != nil {
		return false, err
	}

	return waitForJob(ctx, res.JobID)
}

type RcloneStatInput struct {
	Path paths.Path
}

func RcloneStat(ctx context.Context, input RcloneStatInput) (*rclone.StatResponse, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Starting RcloneStat")

	srcFs, srcRemote := input.Path.RcloneFsRemote()

	res, err := rclone.Stat(srcFs, srcRemote)
	if err != nil {
		return nil, err
	}

	return res, nil
}

// RcloneWaitForFile waits until a file stops growing
// Useful for waiting for a file to be fully uploaded, e.g. watch folders
// Returns true if file is fully uploaded, false if failed, e.g. file doesnt exist after 5 minutes.
func RcloneWaitForFile(ctx context.Context, input RcloneFileInput) (bool, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Starting RcloneWaitForFile")

	srcFs, srcRemote := input.Source.RcloneFsRemote()

	// Use to cancel if file doesnt exist still after 5 minutes
	startedAt := time.Now()

	lastKnownSize := int64(0)
	iterationsWhereSizeIsFreezed := 0

	for {
		res, err := rclone.Stat(
			srcFs, srcRemote,
		)
		activity.RecordHeartbeat(ctx, res)
		if err != nil {
			if time.Since(startedAt) > time.Minute*5 {
				return false, err
			}
			time.Sleep(time.Second * 5)
			continue
		}

		if res.Size < lastKnownSize {
			return false, fmt.Errorf("file size decreased")
		} else if res.Size > lastKnownSize {
			lastKnownSize = res.Size
			iterationsWhereSizeIsFreezed = 0
			time.Sleep(time.Second * 5)
			continue
		}

		iterationsWhereSizeIsFreezed++

		if iterationsWhereSizeIsFreezed > 5 {
			return true, nil
		}
	}
}
