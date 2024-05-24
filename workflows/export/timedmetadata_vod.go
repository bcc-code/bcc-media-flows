package export

import (
	"encoding/json"
	"fmt"

	"github.com/bcc-code/bcc-media-flows/activities"
	vsactivity "github.com/bcc-code/bcc-media-flows/activities/vidispine"
	"github.com/bcc-code/bcc-media-flows/services/rclone"
	"github.com/bcc-code/bcc-media-flows/services/telegram"
	"github.com/bcc-code/bcc-media-flows/services/vidispine"
	wfutils "github.com/bcc-code/bcc-media-flows/utils/workflows"
	"github.com/bcc-code/bcc-media-platform/backend/asset"
	"github.com/bcc-code/bcc-media-platform/backend/events"
	"go.temporal.io/sdk/workflow"
)

type ExportTimedMetadataParams struct {
	VXID       string
	ExportData *vidispine.ExportData
}

type ExportTimedMetadataResult struct {
	VXID string
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

	exportData := params.ExportData
	if exportData == nil {
		err := wfutils.Execute(ctx, vsactivity.Vidispine.GetExportDataActivity, vsactivity.GetExportDataParams{
			VXID:        params.VXID,
			Languages:   []string{"no"},
			AudioSource: vidispine.ExportAudioSourceEmbedded.Value,
		}).Get(ctx, &exportData)
		if err != nil {
			return nil, err
		}
	}

	var timedMetadata *[]asset.TimedMetadata
	err = wfutils.Execute(ctx, activities.Vidispine.GetChapterDataActivity, vsactivity.GetChapterDataParams{
		ExportData: exportData,
	}).Get(ctx, &timedMetadata)
	if err != nil {
		return nil, err
	}

	marshalled, err := json.Marshal(timedMetadata)
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

	message := fmt.Sprintf("🟩 Chapter export to VOD finished for %s (`%s`).\nIt should show up in the linked assets within a few minutes.", params.ExportData.Title, params.VXID)

	wfutils.SendTelegramText(
		ctx,
		telegram.ChatVOD,
		message,
	)

	return &ExportTimedMetadataResult{
		VXID: params.VXID,
	}, nil
}
