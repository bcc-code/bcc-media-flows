package cantemo

import (
	"context"
	"fmt"
	vsactivitiy "github.com/bcc-code/bcc-media-flows/activities/vidispine"
	"github.com/bcc-code/bcc-media-flows/services/cantemo"
	"go.temporal.io/sdk/activity"
	"strings"
	"time"
)

type GetFilesParams struct {
	Path     string
	State    string
	Storages []string
	Page     int
	Query    string
}

func GetFiles(_ context.Context, params GetFilesParams) (*cantemo.GetFilesResult, error) {
	return GetClient().GetFiles(
		params.Path,
		params.State,
		strings.Join(params.Storages, ","),
		params.Page,
		params.Query,
	)
}

type RenameFileParams struct {
	ItemID            string
	ShapeID           string
	SourceStorage     string
	DestinatinStorage string
	NewPath           string
}

func RenameFile(_ context.Context, params *RenameFileParams) (string, error) {
	if params.SourceStorage == params.DestinatinStorage {
		return GetClient().RenameFile(params.ItemID, params.ShapeID, params.SourceStorage, params.DestinatinStorage, params.NewPath)
	}

	return GetClient().MoveFile(params.ItemID, params.ShapeID, params.SourceStorage, params.DestinatinStorage, params.NewPath)
}

func MoveFileWait(ctx context.Context, params *RenameFileParams) (any, error) {
	taskID, err := RenameFile(ctx, params)

	if err != nil {
		return nil, err
	}

	status := "STARTED"

	for status == "STARTED" {
		taskStatus, err := GetTaskInfo(ctx, GetTaskInfoParams{
			TaskID: taskID,
		})

		if err != nil {
			return nil, err
		}

		activity.RecordHeartbeat(ctx, taskStatus)
		time.Sleep(5 * time.Second)

		status = taskStatus.State
	}

	if status != "SUCCESS" {
		return nil, fmt.Errorf("task failed with status: %s", status)
	}

	job, err := vsactivitiy.Vidispine.FindJob(ctx, vsactivitiy.FindJobParams{
		ItemID:  params.ItemID,
		JobType: "MOVE_FILE",
	})

	if err != nil {
		return nil, err
	}

	if job == nil {
		// No job found, don't error out. Likely nothing too do
		return nil, nil
	}

	return vsactivitiy.Vidispine.WaitForJobCompletion(ctx, vsactivitiy.WaitForJobCompletionParams{
		JobID:     job.JobID,
		SleepTime: 20,
	})
}
