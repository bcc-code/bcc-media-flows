package ingestworkflows

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/bcc-code/bcc-media-flows/services/telegram"
	miscworkflows "github.com/bcc-code/bcc-media-flows/workflows/misc"

	"github.com/bcc-code/bcc-media-flows/activities"
	vsactivity "github.com/bcc-code/bcc-media-flows/activities/vidispine"
	"github.com/bcc-code/bcc-media-flows/paths"
	"github.com/bcc-code/bcc-media-flows/services/vidispine/vscommon"
	wfutils "github.com/bcc-code/bcc-media-flows/utils/workflows"
	"go.temporal.io/sdk/workflow"
)

type IncrementalParams struct {
	Path string
}

// Constants for workflow and signal
const (
	LiveIngestWorkflowID  = "LIVE-INGEST"
	FileTransferredSignal = "file_transferred"
)

// Incremental is a workflow that ingests a growing file into Vidispine.
// It also starts the Reaper recording.
//
// The workflow has a fixed ID "LIVE-INGEST" and listens for file transfer signals.
// It will repeatedly attempt to copy files until it receives a signal that the file
// has been completely transferred.
//
// After the ingest is done, it stops the Reaper recording and adds the file to the placeholder.
// The reaper command returns the list of files that were recorded, so we can await for them to be
// available before padding them to the same start as the video file.
// The length of the files will typically be longer than video but that is not an issue.
//
// After the files are modified, they need to be ingested into Vidispine, and
// linked properly to the video file
func Incremental(ctx workflow.Context, params IncrementalParams) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting Incremental with fixed ID: LIVE-INGEST")

	// Override the workflow ID to be LIVE-INGEST
	info := workflow.GetInfo(ctx)
	if info.WorkflowExecution.ID != LiveIngestWorkflowID {
		logger.Warn(fmt.Sprintf("Workflow was started with ID %s instead of %s", info.WorkflowExecution.ID, LiveIngestWorkflowID))
	}

	ctx = workflow.WithActivityOptions(ctx, wfutils.GetDefaultActivityOptions())

	err := doIncremental(ctx, params)
	if err != nil {
		wfutils.SendTelegramText(ctx, telegram.ChatOther, fmt.Sprintf("🟥 Incremental ingest failed\n\n```%s```", err.Error()))
		return err
	}
	return nil
}

func doIncremental(ctx workflow.Context, params IncrementalParams) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting doIncremental")

	in := paths.MustParse(params.Path)

	outDir, err := wfutils.GetWorkflowRawOutputFolder(ctx)
	if err != nil {
		return err
	}

	rawPath := outDir.Append(in.Base())

	// Create a signal channel to listen for file transfer completions
	signalChan := workflow.GetSignalChannel(ctx, FileTransferredSignal)

	// Extract the base filename we're waiting for
	expectedFilename := in.Base()
	logger.Info(fmt.Sprintf("Waiting for signal with filename: %s", expectedFilename))

	// Create placeholder in Vidispine
	var assetResult vsactivity.CreatePlaceholderResult
	err = wfutils.Execute(ctx, activities.Vidispine.CreatePlaceholderActivity, vsactivity.CreatePlaceholderParams{
		Title: in.Base(),
	}).Get(ctx, &assetResult)
	if err != nil {
		return err
	}

	err = wfutils.SetVidispineMeta(ctx, assetResult.AssetID, vscommon.FieldIngested.Value, workflow.Now(ctx).Format(time.RFC3339))
	if err != nil {
		logger.Error("%w", err)
	}

	videoVXID := assetResult.AssetID

	var p any
	// REAPER: Start recording
	err = wfutils.Execute(ctx, activities.Live.StartReaper, p).Get(ctx, nil)
	if err != nil {
		return err
	}

	wfutils.SendTelegramText(ctx, telegram.ChatOther, fmt.Sprintf("🟦 Starting live ingest: https://vault.bcc.media/item/%s", assetResult.AssetID))

	var jobResult vsactivity.FileJobResult
	err = wfutils.Execute(ctx, activities.Vidispine.AddFileToPlaceholder, vsactivity.AddFileToPlaceholderParams{
		AssetID:  videoVXID,
		FilePath: rawPath,
		Growing:  true,
	}).Get(ctx, &jobResult)
	if err != nil {
		return err
	}

	previewPath, err := wfutils.GetWorkflowAuxOutputFolder(ctx)
	if err != nil {
		logger.Error("%w", err)
	}

	previewTempPath, err := wfutils.GetWorkflowTempFolder(ctx)
	if err != nil {
		logger.Error("%w", err)
	}
	previewTempPath.Append("preview")

	previewPath.Append(expectedFilename).SetExt("mp4")

	lowresImportJob, err := wfutils.Execute(ctx, activities.Vidispine.ImportFileAsShapeActivity, vsactivity.ImportFileAsShapeParams{
		AssetID:  videoVXID,
		FilePath: previewPath,
		ShapeTag: "lowres_watermarked",
		Growing:  true,
		Replace:  false,
	}).Result(ctx)

	if err != nil {
		logger.Error("%w", err)
	}

	previewFuture := wfutils.Execute(ctx, activities.Video.TranscodeGrowingPreview, activities.TranscodeGrowingPreviewParams{
		FilePath:           rawPath,
		DestinationDirPath: previewPath,
		TempFolderPath:     previewTempPath,
	})

	signalReceived := false

	// Start listening for signals in the background
	workflow.Go(ctx, func(ctx workflow.Context) {
		for {
			var signalFileName string
			// Wait for a signal
			signalChan.Receive(ctx, &signalFileName)

			logger.Info(fmt.Sprintf("Received file transfer signal for: %s", signalFileName))

			// Check if the signal matches our expected filename
			if strings.EqualFold(filepath.Base(signalFileName), expectedFilename) {
				logger.Info("Signal matches our file, marking as completed")
				signalReceived = true
				return // Exit the goroutine when we get the right signal
			} else {
				logger.Info(fmt.Sprintf("Signal was for a different file: %s, ignoring", signalFileName))
			}
		}
	})

	// Keep copying the file until we receive a signal or the copy process completes naturally
	maxCopyAttempts := 1000 // Limit total number of attempts

	for copyAttempt := 0; copyAttempt < maxCopyAttempts; copyAttempt++ {
		logger.Info(fmt.Sprintf("Starting copy attempt %d", copyAttempt+1))

		copyFuture := wfutils.Execute(ctx, activities.Live.RsyncIncrementalCopy, activities.RsyncIncrementalCopyInput{
			In:  in,
			Out: rawPath,
		})

		err := copyFuture.Wait(ctx)
		if err != nil {
			logger.Error("Copy operation failed", "error", err)
		} else {
			logger.Info("Copy operation completed successfully")

			if !signalReceived {
				logger.Info("Sleeping for 1 minute before next copy attempt")
				_ = workflow.Sleep(ctx, time.Minute)
			} else {
				logger.Info("Received signal, breaking out of copy loop")
				break
			}
		}
	}

	wfutils.SendTelegramText(ctx, telegram.ChatOther, fmt.Sprintf("🟦 Video ingest ended: https://vault.bcc.media/item/%s\n\nImporting reaper files.", assetResult.AssetID))

	// List Reaper files
	reaperResult := &activities.ReaperResult{}
	err = wfutils.Execute(ctx, activities.Live.ListReaperFiles, nil).Get(ctx, reaperResult)
	if err != nil {
		return err
	}

	err = wfutils.Execute(ctx, activities.Vidispine.CloseFile, vsactivity.CloseFileParams{
		FileID: jobResult.FileID,
	}).Get(ctx, nil)
	if err != nil {
		return err
	}

	baseName := strings.TrimSuffix(in.Base(), "_MU1.mxf")

	// Wait for all reaper files to be imported
	var importAudioFuture []workflow.ChildWorkflowFuture
	for _, file := range reaperResult.Files {
		fileSplit := strings.Split(file, "\\")
		filePath := "/mnt/filecatalyst/wavetemp/" + fileSplit[len(fileSplit)-1]
		f := workflow.ExecuteChildWorkflow(ctx, ImportAudioFileFromReaper, ImportAudioFileFromReaperParams{
			Path:       filePath,
			VideoVXID:  videoVXID,
			BaseName:   baseName,
			OutputPath: outDir,
		})

		importAudioFuture = append(importAudioFuture, f)
	}

	// Transcribe the video
	transcribeFuture := workflow.ExecuteChildWorkflow(ctx, miscworkflows.TranscribeVX, miscworkflows.TranscribeVXInput{
		VXID:                videoVXID,
		Language:            "no",
		NotificationChannel: &telegram.ChatOther,
	})

	_ = wfutils.Execute(ctx, activities.Vidispine.CreateThumbnailsActivity, vsactivity.CreateThumbnailsParams{
		AssetID: videoVXID,
	}).Get(ctx, nil)

	var errors []error
	for _, f := range importAudioFuture {
		err = f.Get(ctx, nil)
		if err != nil {
			errors = append(errors, err)
		}
	}

	wfutils.SendTelegramText(ctx, telegram.ChatOther, "🟩 Audio import finished")

	err = transcribeFuture.Get(ctx, nil)
	if err != nil {
		errors = append(errors, err)
	}

	if len(errors) > 0 {
		return fmt.Errorf("failed to import one or more audio files: %v", errors)
	}

	_ = previewFuture.Wait(ctx)
	wfutils.Execute(ctx, activities.Vidispine.CloseFile, vsactivity.CloseFileParams{
		FileID: lowresImportJob.FileID,
	})

	return nil
}
