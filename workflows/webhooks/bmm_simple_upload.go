package webhooks

import (
	"fmt"
	"strconv"

	"github.com/bcc-code/bcc-media-flows/paths"
	"github.com/bcc-code/bcc-media-flows/services/vidispine/vscommon"
	wfutils "github.com/bcc-code/bcc-media-flows/utils/workflows"
	ingestworkflows "github.com/bcc-code/bcc-media-flows/workflows/ingest"
	"go.temporal.io/sdk/workflow"
)

type BmmSimpleUploadParams struct {
	TrackID    int    `json:"trackId"`
	UploadedBy string `json:"uploadedBy"`
	FilePath   string `json:"filePath"`
	Title      string `json:"title"`
	Language   string `json:"language"`
}

type BmmSimpleUploadResult struct {
}

func BmmSimpleUpload(ctx workflow.Context, params BmmSimpleUploadParams) (*BmmSimpleUploadResult, error) {
	workflow.GetLogger(ctx).Info("Starting BmmSimpleUpload")

	path, err := paths.Parse(params.FilePath)
	if err != nil {
		return nil, err
	}

	workflow.GetLogger(ctx).Info("Uploading file to BMM", "path", path)

	outputDir, err := wfutils.GetWorkflowRawOutputFolder(ctx)
	if err != nil {
		return nil, err
	}

	newPath := outputDir.Append(fmt.Sprintf("BMM-%d-%s", params.TrackID, params.Language), path.Ext())

	err = wfutils.MoveFile(ctx, path, newPath)
	if err != nil {
		return nil, err
	}

	res, err := ingestworkflows.ImportFileAsTag(ctx, "original", newPath, "BMM-"+strconv.Itoa(params.TrackID)+" "+params.Language+" - "+params.Title)
	if err != nil {
		return nil, err
	}

	err = ingestworkflows.SetUploadedBy(ctx, res.AssetID, params.UploadedBy)
	if err != nil {
		return nil, err
	}

	err = ingestworkflows.SetUploadJobID(ctx, res.AssetID, workflow.GetInfo(ctx).OriginalRunID)
	if err != nil {
		return nil, err
	}

	err = wfutils.SetVidispineMeta(ctx, res.AssetID, vscommon.FieldLanguagesRecorded.Value, params.Language)
	if err != nil {
		return nil, err
	}

	err = ingestworkflows.CreatePreviews(ctx, []string{res.AssetID})
	if err != nil {
		return nil, err
	}

	return nil, nil
}
