package webhooks

import "go.temporal.io/sdk/workflow"

type BmmSimpleUploadParams struct {
	TrackID  string
	FilePath string
	Title    string
}

type BmmSimpleUploadResult struct {
}

func BmmSimpleUpload(ctx workflow.Context, params BmmSimpleUploadParams) (*BmmSimpleUploadResult, error) {
	workflow.GetLogger(ctx).Info("Starting BmmSimpleUpload")

	return nil, nil
}
