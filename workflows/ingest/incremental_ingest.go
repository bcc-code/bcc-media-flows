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

// Sample holds a timestamp and bytes transferred
type transferSample struct {
	time  time.Time
	bytes int64
}

type IncrementalParams struct {
	Path            string
	ReaperSessionID string
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
	wfutils.UpsertVXID(ctx, assetResult.AssetID)

	err = wfutils.SetVidispineMeta(ctx, assetResult.AssetID, vscommon.FieldIngested.Value, workflow.Now(ctx).Format(time.RFC3339))
	if err != nil {
		logger.Error("%w", err)
	}

	videoVXID := assetResult.AssetID

	// REAPER: Start recording
	reaperSessionID := params.ReaperSessionID

	if reaperSessionID == "" {
		err = wfutils.Execute(ctx, activities.Live.StartReaper, nil).Get(ctx, &reaperSessionID)
		if err != nil {
			wfutils.SendTelegramText(ctx, telegram.ChatOther, fmt.Sprintf("🟦 Unable to start reaper. Start it manually and notify Matjaz!\n\n```%s```", err.Error()))
		}
	} else {
		wfutils.SendTelegramText(ctx, telegram.ChatOther, fmt.Sprintf("🟦 ASSUMING REAPER SESSION: %s", reaperSessionID))
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

	previewPath = previewPath.Append(rawPath.Base()).SetExt("mp4")

	var previewFuture wfutils.Task[any]
	previewCtx, stopPreviewFunc := workflow.WithCancel(ctx)
	previewActivityOpts := wfutils.GetDefaultActivityOptions()
	previewActivityOpts.StartToCloseTimeout = time.Hour * 8
	previewCtx = workflow.WithActivityOptions(previewCtx, previewActivityOpts)

	workflow.Go(ctx, func(ctx workflow.Context) {
		_ = workflow.Sleep(ctx, 1*time.Minute)
		previewFuture = wfutils.Execute(previewCtx, activities.Video.TranscodeGrowingPreview, activities.TranscodeGrowingPreviewParams{
			OriginalFilePath:    rawPath,
			DestinationFilePath: previewPath,
			TempFolderPath:      previewTempPath,
		})

		_ = workflow.Sleep(ctx, 2*time.Minute)
		lowresImportJob, importErr := wfutils.Execute(ctx, activities.Vidispine.ImportFileAsShapeActivity, vsactivity.ImportFileAsShapeParams{
			AssetID:  videoVXID,
			FilePath: previewPath,
			ShapeTag: "lowres_watermarked",
			Growing:  true,
			Replace:  false,
		}).Result(ctx)
		if importErr != nil {
			// Named separately from the enclosing function's err so the failure cannot
			// be hidden by a later check reading the wrong variable.
			logger.Error("Failed to import growing preview as lowres shape", "error", importErr)
		}

		if previewErr := previewFuture.Wait(ctx); previewErr != nil {
			logger.Error("Growing preview transcode failed", "error", previewErr)
		}

		// Only close a file the import actually produced. A failed import leaves
		// lowresImportJob nil, and dereferencing it panics the workflow task — which
		// Temporal then retries indefinitely.
		if lowresImportJob == nil {
			return
		}

		if closeErr := wfutils.Execute(ctx, activities.Vidispine.CloseFile, vsactivity.CloseFileParams{
			FileID: lowresImportJob.FileID,
		}).Wait(ctx); closeErr != nil {
			logger.Error("Failed to close growing lowres file", "error", closeErr)
		}
	})

	signalReceived := false

	// Initialize slice to store transfer samples
	samples := []transferSample{}

	// alertState tracks whether we are currently in alert mode
	alert := &alertState{}

	// Keep copying the file until we receive a signal or the copy process completes naturally
	maxCopyAttempts := 1000 // Limit total number of attempts

	for copyAttempt := 0; copyAttempt < maxCopyAttempts; copyAttempt++ {
		logger.Info(fmt.Sprintf("Starting copy attempt %d", copyAttempt+1))

		copyFuture := wfutils.Execute(ctx, activities.Live.RsyncIncrementalCopy, activities.RsyncIncrementalCopyInput{
			In:  in,
			Out: rawPath,
		})

		copyResult, err := copyFuture.Result(ctx)
		if err != nil {
			logger.Error("Copy operation failed", "error", err)
		} else {
			sample := transferSample{time: workflow.Now(ctx), bytes: copyResult.Size}
			samples = append(samples, sample)

			// Use the function to calculate rate and prune samples
			rate, pruned := CalculateRollingTransferRate(samples, workflow.Now(ctx), windowDuration)
			samples = pruned
			checkTransferRateAndAlert(ctx, rate, pruned, alert)
		}

		// Waiting on the signal and the timer together is what makes the signal
		// act on arrival. A signal sent while the copy above was running is
		// already queued on the channel, so this returns without sleeping at all.
		signalReceived = waitForTransferSignal(ctx, signalChan, expectedFilename, copyRetryInterval)

		if signalReceived {
			logger.Info("Received signal, breaking out of copy loop")
			break
		}
	}

	// The copy at the top of the loop ran before the signal, so whatever was
	// written to the source in between is not here yet.
	if signalReceived {
		logger.Info("Copying once more now that the source is complete")
		if _, copyErr := wfutils.Execute(ctx, activities.Live.RsyncIncrementalCopy, activities.RsyncIncrementalCopyInput{
			In:  in,
			Out: rawPath,
		}).Result(ctx); copyErr != nil {
			logger.Error("Final copy failed", "error", copyErr)
			wfutils.SendTelegramText(ctx, telegram.ChatOther,
				fmt.Sprintf("🟥 The final copy of %s failed, the ingested file may be short: %v", in.Base(), copyErr))
		}
	}

	wfutils.SendTelegramText(ctx, telegram.ChatOther, fmt.Sprintf("🟦 Video ingest ended: https://vault.bcc.media/item/%s\n\nImporting reaper files.", assetResult.AssetID))

	waitForPreviewToCatchUp(ctx, rawPath, previewPath)

	stopPreviewFunc()

	// List Reaper files
	reaperResult := &activities.ReaperResult{}
	listReaperFilesParams := activities.ListReaperFilesParams{
		SessionID: reaperSessionID,
	}
	err = wfutils.Execute(ctx, activities.Live.ListReaperFiles, &listReaperFilesParams).Get(ctx, reaperResult)
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

	ctx = wfutils.WithChildSearchAttributes(ctx, videoVXID)

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

	// Fix duration metadata issues
	fixDurationFuture := workflow.ExecuteChildWorkflow(ctx, miscworkflows.FixDurationVX, miscworkflows.FixDurationVXInput{
		VXID: videoVXID,
	})

	// Handle errors in background
	workflow.Go(ctx, func(ctx workflow.Context) {
		err := fixDurationFuture.Get(ctx, nil)
		if err != nil {
			wfutils.SendTelegramText(ctx, telegram.ChatOther, fmt.Sprintf("🟥 Duration fix failed for %s: %v", videoVXID, err))
		}
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

	if len(importAudioFuture) > 0 {
		audioFiles, err := wfutils.Execute(ctx, activities.Vidispine.GetRelatedAudioFiles, videoVXID).Result(ctx)
		if err != nil {
			wfutils.SendTelegramText(ctx, telegram.ChatOther, fmt.Sprintf("🟥 Audio/video sync skipped for %s, failed to get related audio files: %v", videoVXID, err))
		} else if _, ok := audioFiles["nor"]; ok {
			err = workflow.ExecuteChildWorkflow(ctx, IngestSyncFix, IngestSyncFixParams{
				VXID: videoVXID,
			}).Get(ctx, nil)
			if err != nil {
				wfutils.SendTelegramText(ctx, telegram.ChatOther, fmt.Sprintf("🟥 Audio/video sync failed for %s: %v", videoVXID, err))
			}
		} else {
			wfutils.SendTelegramText(ctx, telegram.ChatOther, fmt.Sprintf("🟧 Audio/video sync skipped for %s: nor audio not found", videoVXID))
		}
	}

	err = transcribeFuture.Get(ctx, nil)
	if err != nil {
		errors = append(errors, err)
	}

	if len(errors) > 0 {
		return fmt.Errorf("failed to import one or more audio files: %v", errors)
	}

	_ = fixDurationFuture.Get(ctx, nil)

	return nil
}

const (
	windowDuration  = time.Duration(3) * time.Minute
	minTransferRate = 5.0 // Mbps
)

// alertState tracks whether we are currently in alert mode
type alertState struct {
	InAlert bool
}

// CalculateRollingTransferRate returns the transfer rate (Mbps) over the last window, always using at least 4 samples if available.
// It also returns the pruned sample slice for efficient memory usage.
func CalculateRollingTransferRate(samples []transferSample, now time.Time, window time.Duration) (rate float64, pruned []transferSample) {
	// Prune samples to only keep those within the window, or the last 4 if fewer
	pruned = samples[:0]
	cutoff := now.Add(-window)
	for _, s := range samples {
		if s.time.After(cutoff) {
			pruned = append(pruned, s)
		}
	}
	if len(pruned) < 4 && len(samples) >= 4 {
		pruned = samples[len(samples)-4:]
	}
	if len(pruned) < 2 {
		return 0, pruned
	}
	first, last := pruned[0], pruned[len(pruned)-1]
	deltaBytes := last.bytes - first.bytes
	deltaSecs := last.time.Sub(first.time).Seconds()
	if deltaSecs <= 0 {
		return 0, pruned
	}
	return float64(deltaBytes) * 8 / deltaSecs / 1_000_000, pruned
}

// checkTransferRateAndAlert manages alert state and sends recovery/alert messages
func checkTransferRateAndAlert(ctx workflow.Context, rateMbps float64, pruned []transferSample, state *alertState) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Rolling transfer rate", "rateMbps", rateMbps, "inAlert", state.InAlert)
	if len(pruned) < 2 {
		return
	}
	first, last := pruned[0], pruned[len(pruned)-1]
	actualWindow := last.time.Sub(first.time)
	if rateMbps < minTransferRate && !state.InAlert {
		wfutils.SendTelegramText(ctx, telegram.ChatOther, fmt.Sprintf("🟥 ALERT: Ingest transfer rate below %.2f Mbps (%.2f Mbps) for at least %v", minTransferRate, rateMbps, actualWindow))
		state.InAlert = true
		_ = wfutils.Execute(ctx, activities.Util.PokeFileCatalyst, nil).Wait(ctx)
	} else if state.InAlert {
		wfutils.SendTelegramText(ctx, telegram.ChatOther, fmt.Sprintf("🟩 RECOVERY: Ingest transfer rate above %.2f Mbps (%.2f Mbps) for at least %v", minTransferRate, rateMbps, actualWindow))
		state.InAlert = false
	}
}

const (
	// previewCatchUpInterval matches how often the preview activity remuxes its
	// segments, which is what makes progress observable.
	previewCatchUpInterval = time.Minute
	maxPreviewCatchUpWaits = 10

	// previewCatchUpTolerance is how far short of the source still counts as done.
	previewCatchUpTolerance = 5.0

	// copyRetryInterval is how long to wait between incremental copies while the
	// source file is still growing.
	copyRetryInterval = time.Minute
)

// waitForTransferSignal waits up to interval for the transfer-complete signal
// for expectedFilename, reporting whether it arrived.
//
// Signals for other files are consumed and ignored. It returns the moment the
// right signal lands rather than at the end of the current copy-and-sleep
// cycle, so the ingest does not keep rsyncing a file that finished up to a
// minute ago.
func waitForTransferSignal(
	ctx workflow.Context,
	signalChan workflow.ReceiveChannel,
	expectedFilename string,
	interval time.Duration,
) bool {
	logger := workflow.GetLogger(ctx)

	// Cancelling the timer when the signal wins means an abandoned timer does
	// not sit in the history until it fires.
	timerCtx, cancelTimer := workflow.WithCancel(ctx)
	defer cancelTimer()

	timer := workflow.NewTimer(timerCtx, interval)

	var received, elapsed bool
	selector := workflow.NewSelector(ctx)
	selector.AddFuture(timer, func(workflow.Future) {
		elapsed = true
	})
	selector.AddReceive(signalChan, func(c workflow.ReceiveChannel, _ bool) {
		var signalFileName string
		c.Receive(ctx, &signalFileName)

		logger.Info(fmt.Sprintf("Received file transfer signal for: %s", signalFileName))

		if strings.EqualFold(filepath.Base(signalFileName), expectedFilename) {
			logger.Info("Signal matches our file, marking as completed")
			received = true
			return
		}
		logger.Info(fmt.Sprintf("Signal was for a different file: %s, ignoring", signalFileName))
	})

	for !received && !elapsed {
		selector.Select(ctx)
	}

	return received
}

// waitForPreviewToCatchUp blocks until the growing preview covers the whole
// source file.
//
// Cancelling the preview kills the tail feeding ffmpeg's stdin, so anything
// tail had not yet written is lost — the end of the recording, however far
// behind the transcode is. A preview that has reached the source duration is
// one where ffmpeg has consumed everything. Bounded, because never cancelling
// would hang the ingest.
func waitForPreviewToCatchUp(ctx workflow.Context, sourceFile, previewFile paths.Path) {
	logger := workflow.GetLogger(ctx)

	source, err := wfutils.Execute(ctx, activities.Audio.AnalyzeFile, activities.AnalyzeFileParams{
		FilePath: sourceFile,
	}).Result(ctx)
	if err != nil {
		logger.Error("Could not measure the source, stopping the preview without waiting", "error", err)
		return
	}

	previousSeconds := -1.0
	for i := 0; i < maxPreviewCatchUpWaits; i++ {
		if sleepErr := workflow.Sleep(ctx, previewCatchUpInterval); sleepErr != nil {
			return
		}

		preview, analyzeErr := wfutils.Execute(ctx, activities.Audio.AnalyzeFile, activities.AnalyzeFileParams{
			FilePath: previewFile,
		}).Result(ctx)
		if analyzeErr != nil {
			// Expected on the first checks of a short ingest: the preview only
			// starts a minute in, and the muxed file does not exist before that.
			logger.Info("Preview not measurable yet", "error", analyzeErr)
			continue
		}

		if preview.TotalSeconds >= source.TotalSeconds-previewCatchUpTolerance {
			logger.Info("Preview has caught up with the source",
				"previewSeconds", preview.TotalSeconds, "sourceSeconds", source.TotalSeconds)
			return
		}

		if preview.TotalSeconds <= previousSeconds {
			logger.Warn("Preview stopped growing while still short of the source",
				"previewSeconds", preview.TotalSeconds, "sourceSeconds", source.TotalSeconds)
			return
		}

		previousSeconds = preview.TotalSeconds
	}

	logger.Warn("Preview did not catch up in time, stopping it anyway",
		"waited", time.Duration(maxPreviewCatchUpWaits)*previewCatchUpInterval,
		"sourceSeconds", source.TotalSeconds)
}
