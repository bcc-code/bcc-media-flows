package activities

import (
	"context"
	"fmt"
	"time"

	"github.com/bcc-code/bccm-flows/services/rclone"
	"go.temporal.io/sdk/activity"
)

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
			if !job.Success {
				return false, fmt.Errorf("rclone job failed: %s", job.Error)
			}
			return job.Success, nil
		}
		time.Sleep(time.Second * 10)
	}
}
