package activities

import (
	"context"
	"time"

	"github.com/bcc-code/bccm-flows/services/rclone"
	"go.temporal.io/sdk/activity"
)

type RcloneCopyInput struct {
	Source      string
	Destination string
}

func RcloneCopy(ctx context.Context, input RcloneCopyInput) (bool, error) {
	activity.RecordHeartbeat(ctx, "Rclone Copy")

	res, err := rclone.Copy(input.Source, input.Destination)
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
