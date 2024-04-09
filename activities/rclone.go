package activities

import (
	"context"
	"fmt"
	"github.com/bcc-code/bcc-media-flows/services/notifications"
	"time"

	"github.com/bcc-code/bcc-media-flows/paths"

	"github.com/bcc-code/bcc-media-flows/services/rclone"
	"go.temporal.io/sdk/activity"
)

type RcloneWaitForJobInput struct {
	JobID                    int
	SendTelegramNotificatios bool
}

func (ua UtilActivities) RcloneWaitForJob(ctx context.Context, input RcloneWaitForJobInput) (bool, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Starting RcloneWaitForJob")

	var msg *notifications.SendResult
	if input.SendTelegramNotificatios {
		m, err := ua.NotifyTelegramChannel(ctx, fmt.Sprintf("Rclone job %d started", input.JobID))
		if err != nil {
			logger.Error("Failed to send telegram notification", err)
		}
		msg = m
	}

	lastNotification := time.Now()
	for {
		job, err := rclone.CheckJobStatus(input.JobID)
		if err != nil {
			return false, err
		}
		activity.RecordHeartbeat(ctx, job)
		if job == nil {
			return false, nil
		}
		if job.Finished {
			if !job.Success {
				if msg != nil {
					ua.updateTelegramMessage(ctx,
						msg.TelegramMessage,
						fmt.Sprintf("Rclone job %d failed", input.JobID),
					)
				}
				return false, fmt.Errorf("rclone job failed: %s", job.Error)
			}

			if msg != nil {
				ua.updateTelegramMessage(ctx,
					msg.TelegramMessage,
					fmt.Sprintf("Rclone job %d is completed", input.JobID),
				)
			}
			return true, nil
		}

		if msg != nil && lastNotification.Add(time.Minute).Before(time.Now()) {
			ua.updateTelegramMessage(ctx,
				msg.TelegramMessage,
				fmt.Sprintf("Rclone job %d is still running", input.JobID),
			)

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
