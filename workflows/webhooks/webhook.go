package webhooks

import (
	"encoding/json"
	"fmt"
	wfutils "github.com/bcc-code/bcc-media-flows/utils/workflows"

	"github.com/orsinium-labs/enum"
	"go.temporal.io/sdk/workflow"
)

type WebHookInput struct {
	Type       string
	Parameters json.RawMessage
}

type WebHookResult struct {
}

type WebHookType = enum.Member[string]

var (
	WebHookBmmSimpleUpload = WebHookType{Value: "bmm_simple_upload"}

	WebHookTypes = enum.New(WebHookBmmSimpleUpload)
)

// WebHook workflow is a workflow that is triggered by a webhook
// Based on the type, different actions will be taken
func WebHook(ctx workflow.Context, input WebHookInput) (*WebHookResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting WebHook workflow")

	webHookType := WebHookTypes.Parse(input.Type)

	if webHookType == nil {
		return nil, fmt.Errorf("unknown webhook type: %s", input.Type)
	}

	switch *webHookType {
	case WebHookBmmSimpleUpload:
		params, err := wfutils.UnmarshalJson[BmmSimpleUploadParams](ctx, input.Parameters)
		if err != nil {
			return nil, err
		}

		err = workflow.ExecuteChildWorkflow(ctx, BmmSimpleUpload, params).Get(ctx, nil)
		if err != nil {
			return nil, err
		}
	}

	return nil, nil
}
