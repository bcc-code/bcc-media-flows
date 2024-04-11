package ingestworkflows

import (
	"fmt"
	"strings"
	"time"

	"github.com/bcc-code/bcc-media-flows/activities"
	vsactivity "github.com/bcc-code/bcc-media-flows/activities/vidispine"
	"github.com/bcc-code/bcc-media-flows/paths"
	"github.com/bcc-code/bcc-media-flows/services/vidispine/vscommon"
	wfutils "github.com/bcc-code/bcc-media-flows/utils/workflows"
	"github.com/bcc-code/bcc-media-flows/workflows"
	"github.com/bcc-code/mediabank-bridge/log"
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

	err := doIncremental(ctx, params)
	if err != nil {
		_ = wfutils.NotifyTelegramChannel(ctx, fmt.Sprintf("🟥 Incremental ingest failed\n\n```%s```", err.Error()))
		return err
	}
	return nil
}

func doIncremental(ctx workflow.Context, params IncrementalParams) error {
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

	copyTask := wfutils.Execute(ctx, activities.Util.RsyncIncrementalCopy, activities.RsyncIncrementalCopyInput{
		In:  in,
		Out: rawPath,
	})

	var assetResult vsactivity.CreatePlaceholderResult
	err = wfutils.Execute(ctx, activities.Vidispine.CreatePlaceholderActivity, vsactivity.CreatePlaceholderParams{
		Title: in.Base(),
	}).Get(ctx, &assetResult)
	if err != nil {
		return err
	}

	// TODO: this value vas empty? Manually set?
	err = wfutils.SetVidispineMeta(ctx, assetResult.AssetID, vscommon.FieldIngested.Value, time.Now().Format(time.RFC3339))
	if err != nil {
		log.L.Error().Err(err).Send()
	}

	videoVXID := assetResult.AssetID

	var p any
	// REAPER: Start recording
	err = wfutils.Execute(ctx, activities.Util.StartReaper, p).Get(ctx, nil)
	if err != nil {
		return err
	}

	_ = wfutils.NotifyTelegramChannel(ctx, fmt.Sprintf("🟦 Starting live ingest: https://vault.bcc.media/item/%s", assetResult.AssetID))

	var jobResult vsactivity.FileJobResult
	err = wfutils.Execute(ctx, activities.Vidispine.AddFileToPlaceholder, vsactivity.AddFileToPlaceholderParams{
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
	_ = wfutils.NotifyTelegramChannel(ctx, fmt.Sprintf("🟦 Video ingest ended: https://vault.bcc.media/item/%s\n\nImporting reaper files.", assetResult.AssetID))

	// List Reaper files
	reaperResult := &activities.ReaperResult{}
	err = wfutils.Execute(ctx, activities.Util.ListReaperFiles, nil).Get(ctx, reaperResult)
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
	transcribeFuture := workflow.ExecuteChildWorkflow(ctx, workflows.TranscribeVX, workflows.TranscribeVXInput{
		VXID:     videoVXID,
		Language: "no",
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
	_ = wfutils.NotifyTelegramChannel(ctx, "🟩 Audio import finished")

	err = transcribeFuture.Get(ctx, nil)
	if err != nil {
		errors = append(errors, err)
	}

	if len(errors) > 0 {
		return fmt.Errorf("failed to import one or more audio files: %v", errors)
	}

	return nil
}
