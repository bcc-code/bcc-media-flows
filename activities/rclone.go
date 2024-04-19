package activities

import (
	"context"
	"fmt"
	"github.com/bcc-code/bcc-media-flows/services/notifications"
	"time"

	"github.com/bcc-code/bcc-media-flows/paths"
	"github.com/bcc-code/bcc-media-flows/services/rclone"
	"github.com/bcc-code/bcc-media-flows/services/telegram"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/temporal"
)

type TelegramNotificationOptions struct {
	ChatID               telegram.Chat
	NotificationInterval time.Duration
	StartNotification    bool
	EndNotification      bool
}

type RcloneWaitForJobInput struct {
	JobID               int
	NotificationOptions *TelegramNotificationOptions
}

func JobFailedErr(err error) error {
	return temporal.NewNonRetryableApplicationError(fmt.Sprintf("rclone job failed: %s", err.Error()), "rclone_job_failed", err)
}

func (ua UtilActivities) RcloneWaitForJob(ctx context.Context, params RcloneWaitForJobInput) (bool, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Starting RcloneWaitForJob")

	jobID := params.JobID
	notificationOptions := params.NotificationOptions

	if notificationOptions == nil {
		notificationOptions = &TelegramNotificationOptions{
			NotificationInterval: 0,
			StartNotification:    false,
			EndNotification:      false,
		}
	}

	tmpl := notifications.Simple{}
	msg, _ := telegram.NewMessage(notificationOptions.ChatID, tmpl)

	if notificationOptions.StartNotification {
		job, err := rclone.CheckJobStatus(jobID)
		if err != nil {
			return false, JobFailedErr(err)
		}

		tmpl.Message = fmt.Sprintf("Rclone job started: %s, Expected ETA: %d s", job.StartTime, job.Output.Eta)
		_ = msg.UpdateWithTemplate(tmpl)
		msg, _ = telegram.Send(msg)
	}

	lastNotification := time.Now()

	for {
		job, err := rclone.CheckJobStatus(jobID)
		if err != nil {
			return false, JobFailedErr(err)
		}
		activity.RecordHeartbeat(ctx, job)
		if job == nil {
			return false, nil
		}
		if job.Finished {

			if notificationOptions.EndNotification {
				tmpl.Message = fmt.Sprintf("Rclone job finished: %s, Duration: %f", job.StartTime, job.Duration)
				msg.UpdateWithTemplate(tmpl)
				msg, _ = telegram.Send(msg)
			}

			if !job.Success {
				return false, JobFailedErr(fmt.Errorf("job failed: %s", job.Output.LastError))
			}

			return true, nil
		}

		if notificationOptions.NotificationInterval > 0 && time.Since(lastNotification) > notificationOptions.NotificationInterval {
			tmpl.Message = fmt.Sprintf("Rclone job running: %s, ETA: %d s", job.StartTime, job.Output.Eta)
			msg.UpdateWithTemplate(tmpl)
			msg, _ = telegram.Send(msg)
			lastNotification = time.Now()
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
