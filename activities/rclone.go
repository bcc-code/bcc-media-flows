package activities

import (
	"context"
	"github.com/bcc-code/bccm-flows/services/rclone"
	"go.temporal.io/sdk/activity"
	"time"
)

type RcloneUploadDirInput struct {
	Source      string
	Destination string
}

func RcloneUploadDir(ctx context.Context, input RcloneUploadDirInput) (bool, error) {
	activity.RecordHeartbeat(ctx, "Rclone Upload Dir")

	res, err := rclone.CopyDir(input.Source, input.Destination)
	if err != nil {
		return false, err
	}

	for {
		job, err := rclone.CheckJobStatus(res.JobID)
		if err != nil {
			return false, err
		}
		activity.RecordHeartbeat(ctx, job)
		if job == nil {
			return false, nil
		}
		if job.Finished {
			return job.Success, nil
		}
		time.Sleep(time.Second * 10)
	}
}
