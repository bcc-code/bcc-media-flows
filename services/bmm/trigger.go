package bmm

import (
	"context"
	"fmt"
	"github.com/bcc-code/bcc-media-flows/internal/bootstrap"

	"github.com/bcc-code/bcc-media-flows/environment"
	wfutils "github.com/bcc-code/bcc-media-flows/utils/workflows"
	ingestworkflows "github.com/bcc-code/bcc-media-flows/workflows/ingest"
	"go.temporal.io/sdk/client"
)

func NewTemporalClient() (client.Client, error) {
	return bootstrap.TemporalClient()
}

type TriggerResult struct {
	WorkflowID string
	RunID      string
	Result     *ingestworkflows.BmmTrackMetadataResult
}

func TriggerBmmTrackMetadata(ctx context.Context, c client.Client, params ingestworkflows.BmmTrackMetadataParams, wait bool, triggeredBy string) (*TriggerResult, error) {
	if triggeredBy == "" {
		triggeredBy = "bmm-trigger"
	}
	opts := wfutils.NewWorkflowOptions(environment.GetQueue(), "", triggeredBy)

	run, err := c.ExecuteWorkflow(ctx, opts, ingestworkflows.BmmTrackMetadata, params)
	if err != nil {
		return nil, fmt.Errorf("failed to start workflow: %w", err)
	}

	out := &TriggerResult{
		WorkflowID: run.GetID(),
		RunID:      run.GetRunID(),
	}

	if wait {
		var result ingestworkflows.BmmTrackMetadataResult
		if err := run.Get(ctx, &result); err != nil {
			return out, fmt.Errorf("workflow failed: %w", err)
		}
		out.Result = &result
	}

	return out, nil
}
