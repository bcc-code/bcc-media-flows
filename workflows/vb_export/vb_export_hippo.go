package vb_export

import (
	"github.com/ansel1/merry/v2"
	"github.com/bcc-code/bccm-flows/activities"
	"github.com/bcc-code/bccm-flows/common"
	"github.com/bcc-code/bccm-flows/paths"
	wfutils "github.com/bcc-code/bccm-flows/utils/workflows"
	"go.temporal.io/sdk/workflow"
)

var (
	ameFlexResPerformanceWatchFolderInput = paths.Path{
		Drive: paths.IsilonDrive,
		Path:  "system/transcodetemp/hippo/hapqhippo/in/",
	}
	ameFlexResPerformanceWatchFolderOutput = paths.Path{
		Drive: paths.IsilonDrive,
		Path:  "system/transcodetemp/hippo/hapqhippo/out/",
	}
	ameFlexResQualityWatchFolderInput = paths.Path{
		Drive: paths.IsilonDrive,
		Path:  "system/transcodetemp/hippo/hapalphahippo/in/",
	}
	ameFlexResQualityWatchFolderOutput = paths.Path{
		Drive: paths.IsilonDrive,
		Path:  "system/transcodetemp/hippo/hapalphahippo/out/",
	}
)

/*
# Requirements

Container: FlexRes
Video: Various resolutions, 25p/50p, FlexRes Performance (default), FlexRes Quality (alpha)
Audio: None
*/
func VBExportToHippo(ctx workflow.Context, params VBExportChildWorkflowParams) (*VBExportResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting ExportToHippo")

	options := wfutils.GetDefaultActivityOptions()
	ctx = workflow.WithActivityOptions(ctx, options)

	hippoOutputDir := params.TempDir.Append("hippo_output")
	err := wfutils.CreateFolder(ctx, hippoOutputDir)
	if err != nil {
		return nil, err
	}

	currentVideoFile := params.InputFile
	if params.SubtitleFile != nil {
		// Burn in subtitle
		var videoResult common.VideoResult
		err = wfutils.ExecuteWithQueue(ctx, activities.TranscodeToProResActivity, activities.EncodeParams{
			FilePath:       currentVideoFile,
			OutputDir:      hippoOutputDir,
			Interlace:      false,
			BurnInSubtitle: params.SubtitleFile,
			Alpha:          params.AnalyzeResult.HasAlpha,
		}).Get(ctx, &videoResult)
		if err != nil {
			return nil, err
		}
		currentVideoFile = videoResult.OutputPath
	}

	var success *bool
	inputFolder := ameFlexResPerformanceWatchFolderInput
	outputFile := ameFlexResPerformanceWatchFolderOutput.Append(params.InputFile.Base())
	if params.AnalyzeResult.HasAlpha {
		inputFolder = ameFlexResQualityWatchFolderInput
		outputFile = ameFlexResQualityWatchFolderOutput.Append(params.InputFile.Base())
	}

	// Rclone to watch-folder
	err = wfutils.ExecuteWithQueue(ctx, activities.RcloneCopyFile, activities.EncodeParams{
		FilePath:  currentVideoFile,
		OutputDir: inputFolder,
	}).Get(ctx, &success)
	if err != nil {
		return nil, err
	}
	if success == nil || !*success {
		return nil, merry.New("RcloneCopyFile failed")
	}

	// Wait for Ame to finish
	err = wfutils.ExecuteWithQueue(ctx, activities.RcloneWaitForFile, activities.RcloneStatInput{
		Path: outputFile,
	}).Get(ctx, &success)

	// Rclone to playout
	/*
		err = wfutils.ExecuteWithQueue(ctx, activities.RcloneMoveFile, activities.RcloneFileInput{
			Source:      ameOutputPath,
			Destination: paths.New(

			),
		}).Get(ctx, nil)
		if err != nil {
			return nil, err
		} */

	return &VBExportResult{
		ID:    params.ParentParams.VXID,
		Title: params.ExportData.SafeTitle,
	}, nil
}
