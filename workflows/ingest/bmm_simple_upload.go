package ingestworkflows

import (
	"fmt"
	"strconv"

	"github.com/bcc-code/bcc-media-flows/activities"
	vsactivity "github.com/bcc-code/bcc-media-flows/activities/vidispine"
	"github.com/bcc-code/bcc-media-flows/paths"
	"github.com/bcc-code/bcc-media-flows/services/rclone"
	"github.com/bcc-code/bcc-media-flows/services/telegram"
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
	IsPodcast                 bool   `json:"isPodcast"`
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

	if params.IsPodcast {
		err = deliverToSSF(ctx, res.AssetID, newPath, params)
		if err != nil {
			wfutils.SendTelegramErorr(ctx, telegram.ChatBMM, res.AssetID, err)
			return nil, err
		}
	}

	wfutils.SendTelegramText(ctx, telegram.ChatBMM, fmt.Sprintf("🟦 Successfully created MB asset `%s`, for language `%s`, uploaded by `%s` ", res.AssetID, params.Language, params.UploadedBy))
	wfutils.SendEmails(ctx, []string{params.UploadedBy}, "BMM Upload successful", fmt.Sprintf("Uploaded file has been imported into Mediabanken. Asset ID: %s\nUploaded by: %s\nLanguage: %s\n", res.AssetID, params.UploadedBy, params.Language))

	return &BmmSimpleUploadResult{}, nil
}

type ssfMetadata struct {
	TrackID  int    `json:"track_id"`
	Title    string `json:"title"`
	Language string `json:"language"`
	AssetID  string `json:"asset_id"`
}

func deliverToSSF(ctx workflow.Context, assetID string, wavPath paths.Path, params BmmSimpleUploadParams) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("Delivering podcast to SSF")

	ssfOutputDir, err := wfutils.GetWorkflowTempFolder(ctx)
	if err != nil {
		return err
	}
	ssfOutputDir = ssfOutputDir.Append("ssf_delivery")

	err = wfutils.CreateFolder(ctx, ssfOutputDir)
	if err != nil {
		return err
	}

	// Copy the original WAV file to SSF output folder
	wavDestination := ssfOutputDir.Append(wavPath.Base())
	err = wfutils.CopyFile(ctx, wavPath, wavDestination)
	if err != nil {
		return fmt.Errorf("failed to copy WAV file: %w", err)
	}

	// Get the transcription JSON from Vidispine
	transcriptResult, err := wfutils.Execute(ctx, activities.Vidispine.GetFileFromVXActivity, vsactivity.GetFileFromVXParams{
		VXID: assetID,
		Tags: []string{"transcription_json"},
	}).Result(ctx)
	if err != nil {
		logger.Warn("Failed to get transcription JSON from Vidispine", "error", err)
		// Continue without transcription
	} else {
		// Copy transcription to SSF output folder
		transcriptDestination := ssfOutputDir.Append(transcriptResult.FilePath.Base())
		err = wfutils.CopyFile(ctx, transcriptResult.FilePath, transcriptDestination)
		if err != nil {
			return fmt.Errorf("failed to copy transcription file: %w", err)
		}
	}

	// Create metadata JSON
	metadata := ssfMetadata{
		TrackID:  params.TrackID,
		Title:    params.Title,
		Language: params.Language,
		AssetID:  assetID,
	}

	metadataJSON, err := wfutils.MarshalJson(ctx, metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata JSON: %w", err)
	}

	err = wfutils.WriteFile(ctx, ssfOutputDir.Append("metadata.json"), metadataJSON)
	if err != nil {
		return fmt.Errorf("failed to write metadata JSON: %w", err)
	}

	// Copy to SSF
	ssfDestination := "s3prod:/bccm-ssf/from-mb/" + fmt.Sprintf("BMM-%d-%s", params.TrackID, params.Language)
	err = wfutils.RcloneCopyDir(ctx, ssfOutputDir.Rclone(), ssfDestination, rclone.PriorityNormal)
	if err != nil {
		return fmt.Errorf("failed to copy to SSF: %w", err)
	}

	wfutils.SendTelegramText(ctx, telegram.ChatBMM, fmt.Sprintf("🟦 Delivered podcast to SSF: `%s`", ssfDestination))

	return nil
}
