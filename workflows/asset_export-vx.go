package workflows

import (
	"github.com/bcc-code/bccm-flows/activities"
	avidispine "github.com/bcc-code/bccm-flows/activities/vidispine"
	"github.com/bcc-code/bccm-flows/common"
	"github.com/bcc-code/bccm-flows/services/vidispine"
	"github.com/bcc-code/bccm-flows/utils"
	"go.temporal.io/sdk/workflow"
)

type AssetExportParams struct {
	VXID string
}

type AssetExportResult struct {
}

func AssetExportVX(ctx workflow.Context, params AssetExportParams) (*AssetExportResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting AssetExport")

	ctx = workflow.WithActivityOptions(ctx, DefaultActivityOptions)

	var data *vidispine.ExportData

	err := workflow.ExecuteActivity(ctx, avidispine.GetExportDataActivity, avidispine.GetExportDataParams{
		VXID: params.VXID,
	}).Get(ctx, &data)
	if err != nil {
		return nil, err
	}

	logger.Info("Retrieved data from vidispine")

	workflowFolder, err := utils.GetWorkflowOutputFolder(ctx)
	if err != nil {
		return nil, err
	}

	mergeInput := common.MergeInput{
		Title:     data.Title,
		OutputDir: workflowFolder,
	}

	var audioMergeInputs = map[string]*common.MergeInput{}

	for _, clip := range data.Clips {
		mergeInput.Items = append(mergeInput.Items, common.MergeInputItem{
			Path:  clip.VideoFile,
			Start: clip.InSeconds,
			End:   clip.OutSeconds,
		})

		for lan, af := range clip.AudioFiles {
			if _, ok := audioMergeInputs[lan]; !ok {
				audioMergeInputs[lan] = &common.MergeInput{
					Title:     data.Title + "-" + lan,
					OutputDir: workflowFolder,
				}
			}

			audioMergeInputs[lan].Items = append(audioMergeInputs[lan].Items, common.MergeInputItem{
				Path:    af.File,
				Start:   clip.InSeconds,
				End:     clip.OutSeconds,
				Streams: af.Channels,
			})
		}
	}

	err = workflow.ExecuteActivity(ctx, activities.TranscodeMergeVideo, mergeInput).Get(ctx, nil)

	if err != nil {
		return nil, err
	}

	for _, mi := range audioMergeInputs {
		err = workflow.ExecuteActivity(ctx, activities.TranscodeMergeAudio, *mi).Get(ctx, nil)
		if err != nil {
			return nil, err
		}
	}

	return nil, nil
}
