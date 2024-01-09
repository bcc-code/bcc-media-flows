package workflows

import (
	"fmt"
	"strings"
	"time"

	"github.com/bcc-code/bcc-media-flows/activities"
	"github.com/bcc-code/bcc-media-flows/environment"
	"github.com/bcc-code/bcc-media-flows/paths"
	wfutils "github.com/bcc-code/bcc-media-flows/utils/workflows"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

type HandleMultitrackFileInput struct {
	Path string
}

func HandleMultitrackFile(
	ctx workflow.Context,
	params HandleMultitrackFileInput,
) error {
	logger := workflow.GetLogger(ctx)
	options := workflow.ActivityOptions{
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval: time.Minute * 1,
			MaximumAttempts: 1,
			MaximumInterval: time.Hour * 1,
		},
		StartToCloseTimeout:    time.Hour * 4,
		ScheduleToCloseTimeout: time.Hour * 48,
		HeartbeatTimeout:       time.Minute * 1,
		TaskQueue:              environment.GetWorkerQueue(),
	}

	ctx = workflow.WithActivityOptions(ctx, options)
	logger.Info("Starting HandleMultitrackFile workflow")

	path, err := paths.Parse(params.Path)
	if err != nil {
		return err
	}

	path, err = wfutils.StandardizeFileName(ctx, path)
	if err != nil {
		return err
	}

	lucidPath := paths.Path{
		Drive: paths.LucidLinkDrive,
		Path:  strings.Replace(path.Dir().Path, "system/multitrack/Ingest/tempFraBrunstad", "", 1),
	}

	lucidPath = lucidPath.Append(path.Base()).Prepend("01 Liveopptak fra Brunstad/01 RAW")

	err = workflow.ExecuteActivity(ctx, activities.CopyFile, activities.MoveFileInput{
		Source:      path,
		Destination: lucidPath,
	}).Get(ctx, nil)
	if err != nil {
		return err
	}

	isilonArchivePath := paths.Path{
		Drive: paths.IsilonDrive,
		Path:  strings.Replace(path.Dir().Path, "system/multitrack/Ingest/tempFraBrunstad", "", 1),
	}.Prepend(fmt.Sprintf("AudioArchive/%d/%d", time.Now().Year(), time.Now().Month())).Append(path.Base())

	err = workflow.ExecuteActivity(ctx, activities.MoveFile, activities.MoveFileInput{
		Source:      path,
		Destination: isilonArchivePath,
	}).Get(ctx, nil)

	return err
}
