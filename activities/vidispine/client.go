package vidispine

import (
	"context"
	"fmt"
	"github.com/davecgh/go-spew/spew"
	"go.temporal.io/sdk/activity"
	"os"
	"time"

	"github.com/bcc-code/bccm-flows/services/vidispine"
	"github.com/bcc-code/bccm-flows/services/vidispine/vsapi"
)

func GetClient() *vidispine.VidispineService {

	vsapiClient := vsapi.NewClient(os.Getenv("VIDISPINE_BASE_URL"), os.Getenv("VIDISPINE_USERNAME"), os.Getenv("VIDISPINE_PASSWORD"))

	return vidispine.NewVidispineService(vsapiClient)
}

type WaitForJobCompletionParams struct {
	JobID string
}

func WaitForJobCompletion(ctx context.Context, params WaitForJobCompletionParams) error {
	logger := activity.GetLogger(ctx)
	logger.Info("Starting WaitForJobCompletionActivity")

	vsClient := GetClient()

	for {
		job, err := vsClient.GetJob(params.JobID)
		if err != nil {
			return err
		}
		if job.Status == "FINISHED" {
			return nil
		}
		if job.Status != "STARTED" && job.Status != "READY" && job.Status != "WAITING" {
			spew.Dump(job)
			return fmt.Errorf("job failed with status: %s", job.Status)
		}
		activity.RecordHeartbeat(ctx, job)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		time.Sleep(time.Second * 30)
	}
}
