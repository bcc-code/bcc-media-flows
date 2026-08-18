package cantemo

import (
	"context"
	"github.com/bcc-code/bcc-media-flows/services/cantemo"
)

type GetTaskInfoParams struct {
	TaskID string
}

func (a Activities) GetTaskInfo(_ context.Context, params GetTaskInfoParams) (*cantemo.Task, error) {
	return a.Client.GetTask(params.TaskID)
}
