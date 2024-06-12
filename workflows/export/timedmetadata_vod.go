package export

import (
	"fmt"

	"github.com/bcc-code/bcc-media-flows/activities"
	platform_activities "github.com/bcc-code/bcc-media-flows/activities/platform"
	vsactivity "github.com/bcc-code/bcc-media-flows/activities/vidispine"
	"github.com/bcc-code/bcc-media-flows/services/rclone"
	"github.com/bcc-code/bcc-media-flows/services/telegram"
	"github.com/bcc-code/bcc-media-flows/services/vidispine"
	wfutils "github.com/bcc-code/bcc-media-flows/utils/workflows"
	"github.com/bcc-code/bcc-media-platform/backend/events"
	"go.temporal.io/sdk/workflow"
)

type ExportTimedMetadataParams struct {
	VXID string
}

type ExportTimedMetadataResult struct {
	VXID  string
	Count int
}

// ExportTimedMetadata exports chapters to VOD as timedmetadata
// After this flow, a job will be triggered in the BCC Media Platform to ingest the chapters.
func ExportTimedMetadata(ctx workflow.Context, params ExportTimedMetadataParams) (*ExportTimedMetadataResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting ExportTimedMetadata")

	ctx = workflow.WithActivityOptions(ctx, wfutils.GetDefaultActivityOptions())

	tempDir, err := wfutils.GetWorkflowTempFolder(ctx)
	if err != nil {
		return nil, err
	}
	outputDir := tempDir.Append("output")

	exportData, err := wfutils.Execute(ctx, vsactivity.Vidispine.GetExportDataActivity, vsactivity.GetExportDataParams{
		VXID:        params.VXID,
		Languages:   []string{"nor"},
		AudioSource: vidispine.ExportAudioSourceEmbedded.Value,
	}).Result(ctx)
	if err != nil {
		return nil, err
	}

	timedMetadata, err := wfutils.Execute(ctx, activities.Platform.GetTimedMetadataChaptersActivity, platform_activities.GetTimedMetadataChaptersParams{
		Clips: exportData.Clips,
	}).Result(ctx)
	if err != nil {
		return nil, err
	}

	if len(timedMetadata) == 0 {
		message := fmt.Sprintf("🟩 Timedmetadata->VOD for `%s` (`%s`) done.\nCount: %d.", exportData.Title, params.VXID, 0)
		wfutils.SendTelegramText(
			ctx,
			telegram.ChatVOD,
			message,
		)
		return &ExportTimedMetadataResult{
			VXID:  params.VXID,
			Count: 0,
		}, nil
	}

	marshalled, err := wfutils.MarshalJson(ctx, timedMetadata)
	if err != nil {
		return nil, err
	}
	err = wfutils.WriteFile(ctx, outputDir.Append("timedmetadata.json"), marshalled)
	if err != nil {
		return nil, err
	}

	// Copies created files and any remaining files needed.
	s3Dir := fmt.Sprintf("timedmetadata/%s", params.VXID)
	err = wfutils.RcloneCopyDir(ctx, outputDir.Rclone(), "s3prod:vod-asset-ingest-prod/"+s3Dir, rclone.PriorityNormal)
	if err != nil {
		return nil, err
	}

	err = wfutils.PublishEvent(ctx, events.TypeAssetTimedMetadataDelivered, events.AssetTimedMetadataDelivered{
		VXID:     params.VXID,
		JSONPath: s3Dir + "/timedmetadata.json",
	})
	if err != nil {
		return nil, err
	}

	message := fmt.Sprintf("🟩 Timedmetadata->VOD for `%s` (`%s`) done.\nCount: %d. They should show up in the linked assets within a few minutes.", exportData.Title, params.VXID, len(timedMetadata))

	wfutils.SendTelegramText(
		ctx,
		telegram.ChatVOD,
		message,
	)

	return &ExportTimedMetadataResult{
		VXID:  params.VXID,
		Count: len(timedMetadata),
	}, nil
}
