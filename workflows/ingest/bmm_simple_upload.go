package ingestworkflows

import (
	"fmt"
	"strconv"

	"github.com/bcc-code/bcc-media-flows/services/telegram"

	"github.com/bcc-code/bcc-media-flows/services/rclone"

	"github.com/bcc-code/bcc-media-flows/paths"
	"github.com/bcc-code/bcc-media-flows/services/vidispine/vscommon"
	wfutils "github.com/bcc-code/bcc-media-flows/utils/workflows"
	"github.com/bcc-code/bcc-media-flows/workflows/export"
	miscworkflows "github.com/bcc-code/bcc-media-flows/workflows/misc"
	"go.temporal.io/sdk/workflow"
)

type BmmSimpleUploadParams struct {
	TrackID                   int    `json:"trackId"`
	UploadedBy                string `json:"uploadedBy"`
	FilePath                  string `json:"filePath"`
	Title                     string `json:"title"`
	Language                  string `json:"language"`
	BmmTargetEnvionment       string `json:"bmmTargetEnvironment"`
	ForceReplaceTranscription bool   `json:"forceReplaceTranscription"`
}

type BmmSimpleUploadResult struct {
}

func BmmIngestUpload(ctx workflow.Context, params BmmSimpleUploadParams) (*BmmSimpleUploadResult, error) {
	workflow.GetLogger(ctx).Info("Starting BmmSimpleUpload")

	path := paths.MustParse(params.FilePath)
	ctx = workflow.WithActivityOptions(ctx, wfutils.GetDefaultActivityOptions())

	workflow.GetLogger(ctx).Info("Uploading file to BMM", "path", path)

	wfutils.SendTelegramText(ctx, telegram.ChatBMM, fmt.Sprintf("🟦 Importing file to MB: `%d-%s`", params.TrackID, params.Language))

	outputDir, err := wfutils.GetWorkflowRawOutputFolder(ctx)
	if err != nil {
		return nil, err
	}

	newPath := outputDir.Append(fmt.Sprintf("BMM-%d-%s%s", params.TrackID, params.Language, path.Ext()))

	err = wfutils.MoveFile(ctx, path, newPath, rclone.PriorityNormal)
	if err != nil {
		wfutils.SendTelegramErorr(ctx, telegram.ChatBMM, "", err)
		return nil, err
	}

	res, err := ImportFileAsTag(ctx, "original", newPath, "BMM-"+strconv.Itoa(params.TrackID)+" "+params.Language+" - "+params.Title)
	if err != nil {
		wfutils.SendTelegramErorr(ctx, telegram.ChatBMM, "", err)
		return nil, err
	}

	err = SetUploadedBy(ctx, res.AssetID, params.UploadedBy)
	if err != nil {
		wfutils.SendTelegramErorr(ctx, telegram.ChatBMM, res.AssetID, err)
		return nil, err
	}

	err = SetUploadJobID(ctx, res.AssetID, workflow.GetInfo(ctx).OriginalRunID)
	if err != nil {
		wfutils.SendTelegramErorr(ctx, telegram.ChatBMM, res.AssetID, err)
		return nil, err
	}

	err = wfutils.SetVidispineMeta(ctx, res.AssetID, vscommon.FieldLanguagesRecorded.Value, params.Language)
	if err != nil {
		wfutils.SendTelegramErorr(ctx, telegram.ChatBMM, res.AssetID, err)
		return nil, err
	}

	err = wfutils.SetVidispineMetaInGroup(ctx, res.AssetID, vscommon.FieldBmmTrackID.Value, strconv.Itoa(params.TrackID), "BMM Metadata")
	if err != nil {
		wfutils.SendTelegramErorr(ctx, telegram.ChatBMM, res.AssetID, err)
		return nil, err
	}

	err = wfutils.SetVidispineMetaInGroup(ctx, res.AssetID, vscommon.FieldBmmTitle.Value, params.Title, "BMM Metadata")
	if err != nil {
		wfutils.SendTelegramErorr(ctx, telegram.ChatBMM, res.AssetID, err)
		return nil, err
	}

	err = wfutils.WaitForVidispineJob(ctx, res.ImportJobID)
	if err != nil {
		wfutils.SendTelegramErorr(ctx, telegram.ChatBMM, res.AssetID, err)
		return nil, err
	}

	err = workflow.ExecuteChildWorkflow(ctx, miscworkflows.TranscribeVX, miscworkflows.TranscribeVXInput{
		VXID:     res.AssetID,
		Language: params.Language,
	}).Get(ctx, nil)
	if err != nil {
		wfutils.SendTelegramErorr(ctx, telegram.ChatBMM, res.AssetID, err)
		// Continue. Just because we failed transcription we should not stop the workflow
	}

	destinations := []string{export.AssetExportDestinationBMM.Value}
	if params.BmmTargetEnvionment == "bmm-int" {
		destinations = []string{export.AssetExportDestinationBMMIntegration.Value}
	}

	future := workflow.ExecuteChildWorkflow(ctx, export.VXExport, export.VXExportParams{
		VXID:                      res.AssetID,
		Destinations:              destinations,
		Languages:                 []string{params.Language},
		ForceReplaceTranscription: params.ForceReplaceTranscription,
	})

	_ = CreatePreviews(ctx, []string{res.AssetID})

	err = future.Get(ctx, nil)
	if err != nil {
		wfutils.SendTelegramErorr(ctx, telegram.ChatBMM, res.AssetID, err)
		return nil, err
	}

	wfutils.SendTelegramText(ctx, telegram.ChatBMM, fmt.Sprintf("🟦 Successfully created MB asset `%s`, for language `%s`, uploaded by `%s` ", res.AssetID, params.Language, params.UploadedBy))
	wfutils.SendEmails(ctx, []string{params.UploadedBy}, "BMM Upload successful", fmt.Sprintf("Uploaded file has been imported into Mediabanken. Asset ID: %s\nUploaded by: %s\nLanguage: %s\n", res.AssetID, params.UploadedBy, params.Language))

	return &BmmSimpleUploadResult{}, nil
}
