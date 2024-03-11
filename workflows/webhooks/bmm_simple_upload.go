package webhooks

import (
	"fmt"
	"strconv"

	"github.com/bcc-code/bcc-media-flows/paths"
	"github.com/bcc-code/bcc-media-flows/services/vidispine/vscommon"
	wfutils "github.com/bcc-code/bcc-media-flows/utils/workflows"
	"github.com/bcc-code/bcc-media-flows/workflows"
	"github.com/bcc-code/bcc-media-flows/workflows/export"
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

	ctx = workflow.WithActivityOptions(ctx, wfutils.GetDefaultActivityOptions())

	workflow.GetLogger(ctx).Info("Uploading file to BMM", "path", path)

	outputDir, err := wfutils.GetWorkflowRawOutputFolder(ctx)
	if err != nil {
		return nil, err
	}

	newPath := outputDir.Append(fmt.Sprintf("BMM-%d-%s%s", params.TrackID, params.Language, path.Ext()))

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

	err = wfutils.SetVidispineMetaInGroup(ctx, res.AssetID, vscommon.FieldBmmTrackID.Value, strconv.Itoa(params.TrackID), "BMM Metadata")
	if err != nil {
		return nil, err
	}

	err = wfutils.SetVidispineMetaInGroup(ctx, res.AssetID, vscommon.FieldBmmTitle.Value, params.Title, "BMM Metadata")
	if err != nil {
		return nil, err
	}

	err = wfutils.WaitForVidispineJob(ctx, res.ImportJobID)
	if err != nil {
		return nil, err
	}

	err = workflow.ExecuteChildWorkflow(ctx, workflows.TranscribeVX, workflows.TranscribeVXInput{
		VXID:     res.AssetID,
		Language: params.Language,
	}).Get(ctx, nil)
	if err != nil {
		return nil, err
	}

	future := workflow.ExecuteChildWorkflow(ctx, export.VXExport, export.VXExportParams{
		VXID:         res.AssetID,
		Destinations: []string{"bmm"},
		Languages:    []string{params.Language},
	})

	_ = ingestworkflows.CreatePreviews(ctx, []string{res.AssetID})

	err = future.Get(ctx, nil)
	if err != nil {
		return nil, err
	}

	return &BmmSimpleUploadResult{}, nil
}
