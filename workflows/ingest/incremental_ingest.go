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
	"go.temporal.io/sdk/temporal"
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

	videoVXID, err := createGrowingPlaceholder(ctx, in)
	if err != nil {
		return err
	}

	reaperSessionID := startReaperSession(ctx, params.ReaperSessionID)

	wfutils.SendTelegramText(ctx, telegram.ChatOther, fmt.Sprintf("🟦 Starting live ingest: https://vault.bcc.media/item/%s", videoVXID))

	jobResult, err := wfutils.Execute(ctx, activities.Vidispine.AddFileToPlaceholder, vsactivity.AddFileToPlaceholderParams{
		AssetID:  videoVXID,
		FilePath: rawPath,
		Growing:  true,
	}).Result(ctx)
	if err != nil {
		return err
	}

	previewPath, stopPreview := startGrowingPreview(ctx, rawPath, videoVXID)

	copyUntilTransferred(ctx, in, rawPath, signalChan, expectedFilename)

	wfutils.SendTelegramText(ctx, telegram.ChatOther, fmt.Sprintf("🟦 Video ingest ended: https://vault.bcc.media/item/%s\n\nImporting reaper files.", videoVXID))

	waitForPreviewToCatchUp(ctx, rawPath, previewPath)

	stopPreview()

	reaperResult, err := listReaperFiles(ctx, reaperSessionID, videoVXID)
	if err != nil {
		return err
	}

	err = wfutils.Execute(ctx, activities.Vidispine.CloseFile, vsactivity.CloseFileParams{
		FileID: jobResult.FileID,
	}).Wait(ctx)
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
	}).Wait(ctx)

	var errors []error
	for _, f := range importAudioFuture {
		err = f.Get(ctx, nil)
		if err != nil {
			errors = append(errors, err)
		}
	}

	if len(importAudioFuture) > 0 {
		wfutils.SendTelegramText(ctx, telegram.ChatOther, "🟩 Audio import finished")
		syncAudioToVideo(ctx, videoVXID)
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

// syncAudioToVideo lines the reaper audio up with the video. Every failure here is
// reported and swallowed: the ingest itself has succeeded by this point, and a sync
// that did not happen is something an operator fixes rather than a reason to fail.
func syncAudioToVideo(ctx workflow.Context, videoVXID string) {
	audioFiles, err := wfutils.Execute(ctx, activities.Vidispine.GetRelatedAudioFiles, videoVXID).Result(ctx)
	if err != nil {
		wfutils.SendTelegramText(ctx, telegram.ChatOther,
			fmt.Sprintf("🟥 Audio/video sync skipped for %s, failed to get related audio files: %v", videoVXID, err))
		return
	}

	if _, ok := audioFiles["nor"]; !ok {
		wfutils.SendTelegramText(ctx, telegram.ChatOther,
			fmt.Sprintf("🟧 Audio/video sync skipped for %s: nor audio not found", videoVXID))
		return
	}

	err = workflow.ExecuteChildWorkflow(ctx, IngestSyncFix, IngestSyncFixParams{
		VXID: videoVXID,
	}).Get(ctx, nil)
	if err != nil {
		wfutils.SendTelegramText(ctx, telegram.ChatOther,
			fmt.Sprintf("🟥 Audio/video sync failed for %s: %v", videoVXID, err))
	}
}

func createGrowingPlaceholder(ctx workflow.Context, in paths.Path) (string, error) {
	logger := workflow.GetLogger(ctx)

	assetResult, err := wfutils.Execute(ctx, activities.Vidispine.CreatePlaceholderActivity, vsactivity.CreatePlaceholderParams{
		Title: in.Base(),
	}).Result(ctx)
	if err != nil {
		return "", err
	}

	wfutils.UpsertVXID(ctx, assetResult.AssetID)

	err = wfutils.SetVidispineMeta(ctx, assetResult.AssetID, vscommon.FieldIngested.Value, workflow.Now(ctx).Format(time.RFC3339))
	if err != nil {
		logger.Error("%w", err)
	}

	return assetResult.AssetID, nil
}

// startReaperSession returns the session to take audio from. A failure to start one is
// reported and not fatal: the video ingest is worth finishing without the audio.
func startReaperSession(ctx workflow.Context, existing string) string {
	if existing != "" {
		wfutils.SendTelegramText(ctx, telegram.ChatOther, fmt.Sprintf("🟦 ASSUMING REAPER SESSION: %s", existing))
		return existing
	}

	sessionID, err := wfutils.Execute(ctx, activities.Live.StartReaper, nil).Result(ctx)
	if err != nil {
		wfutils.SendTelegramText(ctx, telegram.ChatOther, fmt.Sprintf("🟦 Unable to start reaper. Start it manually and notify Matjaz!\n\n```%s```", err.Error()))
	}

	return sessionID
}

// startGrowingPreview transcodes a preview alongside the ingest and imports it as the
// lowres shape, so the item is watchable while the source is still arriving. It returns
// where the preview is written and the cancel that stops the transcode.
func startGrowingPreview(ctx workflow.Context, rawPath paths.Path, videoVXID string) (paths.Path, func()) {
	logger := workflow.GetLogger(ctx)

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
	previewCtx, stopPreview := workflow.WithCancel(ctx)
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

	return previewPath, stopPreview
}

// copyUntilTransferred copies the growing source until the watcher signals that the
// transfer finished, then copies once more: the last copy ran before the signal, so
// whatever was written in between is not here yet.
func copyUntilTransferred(ctx workflow.Context, in, rawPath paths.Path, signalChan workflow.ReceiveChannel, expectedFilename string) {
	logger := workflow.GetLogger(ctx)

	samples := []transferSample{}
	alert := &alertState{}
	signalReceived := false

	const maxCopyAttempts = 1000

	for copyAttempt := 0; copyAttempt < maxCopyAttempts; copyAttempt++ {
		logger.Info(fmt.Sprintf("Starting copy attempt %d", copyAttempt+1))

		copyResult, err := wfutils.Execute(ctx, activities.Live.RsyncIncrementalCopy, activities.RsyncIncrementalCopyInput{
			In:  in,
			Out: rawPath,
		}).Result(ctx)
		if err != nil {
			logger.Error("Copy operation failed", "error", err)
		} else {
			samples = append(samples, transferSample{time: workflow.Now(ctx), bytes: copyResult.Size})

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

	if !signalReceived {
		return
	}

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

const (
	windowDuration  = time.Duration(3) * time.Minute
	minTransferRate = 5.0 // Mbps
)

// alertState tracks whether we are currently in alert mode
type alertState struct {
	InAlert bool
}

// listReaperFiles returns the audio the reaper recorded alongside this video.
//
// An empty session id means the reaper never started — doIncremental reports that and
// carries on, because the video ingest is worth finishing without the audio. There is
// nothing to list in that case: the reaper answers a request for a session it does not
// have with an error, or worse, with somebody else's recording. The result is empty, so
// no audio is imported and the audio/video sync that follows it is skipped too.
func listReaperFiles(ctx workflow.Context, sessionID, videoVXID string) (*activities.ReaperResult, error) {
	if sessionID == "" {
		wfutils.SendTelegramText(ctx, telegram.ChatOther,
			fmt.Sprintf("🟧 No reaper session for %s, skipping the audio import. The video is unaffected.", videoVXID))
		return &activities.ReaperResult{}, nil
	}

	result := &activities.ReaperResult{}
	err := wfutils.Execute(ctx, activities.Live.ListReaperFiles, &activities.ListReaperFilesParams{
		SessionID: sessionID,
	}).Get(ctx, result)
	if err != nil {
		return nil, err
	}

	return result, nil
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

	// previewCatchUpDeadline bounds the whole wait in workflow time, rather than
	// counting iterations: a probe can take as long as previewProbeTimeout, so a
	// fixed number of them says nothing about how long the ingest is held open.
	previewCatchUpDeadline = 10 * time.Minute

	// previewProbeTimeout caps one measurement. The probe runs with no retries,
	// so this is also the whole cost of a failed one — retries would spend the
	// deadline on backoff instead of on watching the preview grow.
	previewProbeTimeout = 2 * time.Minute

	// previewCatchUpStaleSamples is how many consecutive measurements must report
	// no progress before the transcode counts as stalled.
	//
	// More than one, because the preview remux is synchronous and takes longer as
	// the recording grows: a probe that lands inside one sees the duration it saw
	// last time while ffmpeg is still working. Treating that as a stall cancels
	// the tail and truncates the preview, which is the thing this wait exists to
	// prevent.
	previewCatchUpStaleSamples = 3

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

	// A failed probe means the preview is missing or mid-remux, which the next
	// cycle answers; the workflow's own policy would retry it ten times with
	// backoff first, spending the deadline below rather than the interval it
	// looks like it spends.
	probeCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout:    previewProbeTimeout,
		ScheduleToCloseTimeout: previewProbeTimeout,
		RetryPolicy:            &temporal.RetryPolicy{MaximumAttempts: 1},
	})

	source, err := wfutils.Execute(probeCtx, activities.Audio.AnalyzeFile, activities.AnalyzeFileParams{
		FilePath: sourceFile,
	}).Result(probeCtx)
	if err != nil {
		logger.Error("Could not measure the source, stopping the preview without waiting", "error", err)
		return
	}

	deadline := workflow.Now(ctx).Add(previewCatchUpDeadline)

	previousSeconds := -1.0
	staleSamples := 0

	for workflow.Now(ctx).Before(deadline) {
		if sleepErr := workflow.Sleep(ctx, previewCatchUpInterval); sleepErr != nil {
			return
		}

		preview, analyzeErr := wfutils.Execute(probeCtx, activities.Audio.AnalyzeFile, activities.AnalyzeFileParams{
			FilePath: previewFile,
		}).Result(probeCtx)
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

		if preview.TotalSeconds > previousSeconds {
			previousSeconds = preview.TotalSeconds
			staleSamples = 0
			continue
		}

		staleSamples++
		if staleSamples >= previewCatchUpStaleSamples {
			logger.Warn("Preview stopped growing while still short of the source",
				"previewSeconds", preview.TotalSeconds, "sourceSeconds", source.TotalSeconds,
				"staleSamples", staleSamples)
			return
		}

		logger.Info("Preview reported no progress, probably mid-remux",
			"previewSeconds", preview.TotalSeconds, "staleSamples", staleSamples)
	}

	logger.Warn("Preview did not catch up in time, stopping it anyway",
		"deadline", previewCatchUpDeadline,
		"previewSeconds", previousSeconds,
		"sourceSeconds", source.TotalSeconds)
}
