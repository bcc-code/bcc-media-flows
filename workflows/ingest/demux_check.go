package ingestworkflows

import (
	"github.com/bcc-code/bcc-media-flows/paths"
	"github.com/bcc-code/bcc-media-flows/services/telegram"
	wfutils "github.com/bcc-code/bcc-media-flows/utils/workflows"
	miscworkflows "github.com/bcc-code/bcc-media-flows/workflows/misc"
	"go.temporal.io/sdk/workflow"
)

// demuxCheckBeforeImport reads the files through ffmpeg before they are handed
// to Mediabanken and reports what it found: by mail to the recipients, or as an
// HTML report next to each file when nobody is known.
//
// It waits for the check, so the report exists before the import starts, but
// it never stops the import: a broken file is still imported and the report
// says what is wrong with it. A check that could not run is logged and
// reported to Telegram, and the import goes on.
func demuxCheckBeforeImport(ctx workflow.Context, files []paths.Path, recipients []string) {
	if len(files) == 0 {
		return
	}
	logger := workflow.GetLogger(ctx)

	// No asset exists yet, so there is no VXID to tag the child with.
	childCtx := wfutils.WithChildSearchAttributes(ctx, "")

	var result miscworkflows.DemuxCheckResult
	err := workflow.ExecuteChildWorkflow(childCtx, miscworkflows.DemuxCheck, miscworkflows.DemuxCheckInput{
		Files:      files,
		Recipients: recipients,
	}).Get(ctx, &result)
	if err != nil {
		logger.Error("Demux check before import failed", "files", len(files), "error", err)
		wfutils.SendTelegramError(ctx, telegram.ChatOther, "", err)
		return
	}

	logger.Info("Demux check before import finished", "outcome", result.Outcome, "files", len(files), "mailed", result.Mailed)
}
