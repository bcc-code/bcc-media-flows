package ingestworkflows

import (
	"fmt"
	"strings"

	"github.com/bcc-code/bcc-media-flows/activities"
	vsactivity "github.com/bcc-code/bcc-media-flows/activities/vidispine"
	"github.com/bcc-code/bcc-media-flows/paths"
	wfutils "github.com/bcc-code/bcc-media-flows/utils/workflows"
	"go.temporal.io/sdk/workflow"
)

type IncrementalParams struct {
	Path string
}

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

	copyTask := wfutils.ExecuteWithQueue(ctx, activities.RsyncIncrementalCopy, activities.RsyncIncrementalCopyInput{
		In:  in,
		Out: rawPath,
	})

	var assetResult vsactivity.CreatePlaceholderResult
	err = wfutils.ExecuteWithQueue(ctx, vsactivity.CreatePlaceholderActivity, vsactivity.CreatePlaceholderParams{
		Title: in.Base(),
	}).Get(ctx, &assetResult)
	if err != nil {
		return err
	}
	wfutils.NotifyTelegramChannel(ctx, fmt.Sprintf("Starting live ingest: https://vault.bcc.media/item/%s", assetResult.AssetID))

	err = wfutils.ExecuteWithQueue(ctx, activities.StartReaper, nil).Get(ctx, nil)
	if err != nil {
		return err
	}
	wfutils.NotifyTelegramChannel(ctx, "Reaper recording started")

	var jobResult vsactivity.FileJobResult
	err = wfutils.ExecuteWithQueue(ctx, vsactivity.AddFileToPlaceholder, vsactivity.AddFileToPlaceholderParams{
		AssetID:  assetResult.AssetID,
		FilePath: rawPath,
		Growing:  true,
	}).Get(ctx, &jobResult)
	if err != nil {
		return err
	}

	err = copyTask.Get(ctx, nil)
	if err != nil {
		return err
	}
	wfutils.NotifyTelegramChannel(ctx, fmt.Sprintf("Video ingest ended: https://vault.bcc.media/item/%s", assetResult.AssetID))

	reaperResult := &activities.StopReaperResult{}
	err = wfutils.ExecuteWithQueue(ctx, activities.StopReaper, nil).Get(ctx, reaperResult)
	if err != nil {
		return err
	}
	wfutils.NotifyTelegramChannel(ctx, fmt.Sprintf("Reaper recording stopped: %s", strings.Join(reaperResult.Files, ", ")))

	err = wfutils.ExecuteWithQueue(ctx, vsactivity.CloseFile, vsactivity.CloseFileParams{
		FileID: jobResult.FileID,
	}).Get(ctx, nil)
	if err != nil {
		return err
	}

	return nil
}
