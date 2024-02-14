package ingestworkflows

import (
	"fmt"
	"strings"

	"github.com/bcc-code/bcc-media-flows/activities"
	vsactivity "github.com/bcc-code/bcc-media-flows/activities/vidispine"
	"github.com/bcc-code/bcc-media-flows/paths"
	wfutils "github.com/bcc-code/bcc-media-flows/utils/workflows"
	"github.com/bcc-code/bcc-media-flows/workflows"
	"go.temporal.io/sdk/workflow"
)

type IncrementalParams struct {
	Path string
}

// Incremental is a workflow that ingests a growing file into Vidispine.
// It also starts the Reaper recording.
//
// After the ingest is done, it stops the Reaper recording and adds the file to the placeholder.
// The reaper command returns the list of files that were recorded, so we can await for them to be
// available before padding them to the same start as the video file.
// The length of the files will typically be longer than video but that is not an issue.
//
// After the files are mdified, the need to be ingested into Vidispine, and
// linked properly to the video file
func Incremental(ctx workflow.Context, params IncrementalParams) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting Incremental")

	ctx = workflow.WithActivityOptions(ctx, wfutils.GetDefaultActivityOptions())

	in, err := paths.Parse(params.Path)
	if err != nil {
		return err
	}

	outDir, err := wfutils.GetWorkflowRawOutputFolder(ctx)
	if err != nil {
		return err
	}

	rawPath := outDir.Append(in.Base())

	/// Start file copy

	copyTask := wfutils.Execute(ctx, activities.RsyncIncrementalCopy, activities.RsyncIncrementalCopyInput{
		In:  in,
		Out: rawPath,
	})

	var assetResult vsactivity.CreatePlaceholderResult
	err = wfutils.Execute(ctx, vsactivity.CreatePlaceholderActivity, vsactivity.CreatePlaceholderParams{
		Title: in.Base(),
	}).Get(ctx, &assetResult)
	if err != nil {
		return err
	}
	wfutils.NotifyTelegramChannel(ctx, fmt.Sprintf("Starting live ingest: https://vault.bcc.media/item/%s", assetResult.AssetID))
	videoVXID := assetResult.AssetID

	// REAPER: Start recording
	err = wfutils.Execute(ctx, activities.StartReaper, nil).Get(ctx, nil)
	if err != nil {
		return err
	}
	wfutils.NotifyTelegramChannel(ctx, "Reaper recording started")

	var jobResult vsactivity.FileJobResult
	err = wfutils.Execute(ctx, vsactivity.AddFileToPlaceholder, vsactivity.AddFileToPlaceholderParams{
		AssetID:  videoVXID,
		FilePath: rawPath,
		Growing:  true,
	}).Get(ctx, &jobResult)
	if err != nil {
		return err
	}

	// Wait for file to be copied
	err = copyTask.Get(ctx, nil)
	if err != nil {
		return err
	}
	wfutils.NotifyTelegramChannel(ctx, fmt.Sprintf("Video ingest ended: https://vault.bcc.media/item/%s", assetResult.AssetID))

	// Stop Reaper recording
	reaperResult := &activities.StopReaperResult{}
	err = wfutils.Execute(ctx, activities.StopReaper, nil).Get(ctx, reaperResult)
	if err != nil {
		return err
	}
	wfutils.NotifyTelegramChannel(ctx, "Reaper recording stopped")

	err = wfutils.Execute(ctx, vsactivity.CloseFile, vsactivity.CloseFileParams{
		FileID: jobResult.FileID,
	}).Get(ctx, nil)
	if err != nil {
		return err
	}

	baseName := strings.TrimSuffix(in.Base(), "_MU1.mxf")

	// Wait for all reaper files to be imported
	importAudioFuture := []workflow.ChildWorkflowFuture{}
	for _, file := range reaperResult.Files {
		fileSplit := strings.Split(file, "\\")
		filePath := "/mnt/dmzshare/wavetemp/" + fileSplit[len(fileSplit)-1]
		f := workflow.ExecuteChildWorkflow(ctx, ImportAudioFileFromReaper, ImportAudioFileFromReaperParams{
			Path:      filePath,
			VideoVXID: videoVXID,
			BaseName:  baseName,
		})

		importAudioFuture = append(importAudioFuture, f)
	}

	// Transcribe the video
	transcribeFuture := workflow.ExecuteChildWorkflow(ctx, workflows.TranscribeVX, workflows.TranscribeVXInput{
		VXID:     videoVXID,
		Language: "no",
	})

	errors := []error{}
	for _, f := range importAudioFuture {
		err = f.Get(ctx, nil)
		if err != nil {
			errors = append(errors, err)
		}
	}
	wfutils.NotifyTelegramChannel(ctx, "Audio import finished")

	err = transcribeFuture.Get(ctx, nil)
	if err != nil {
		errors = append(errors, err)
	}

	if len(errors) > 0 {
		return fmt.Errorf("Failed to import one or more audio files: %v", errors)
	}

	return nil
}
