package cantemo

import (
	"context"
	"github.com/bcc-code/bcc-media-flows/services/cantemo"
)

type GetTaskInfoParams struct {
	TaskID string
}

func GetTaskInfo(_ context.Context, params GetTaskInfoParams) (*cantemo.Task, error) {
	return GetClient().GetTask(params.TaskID)
}
