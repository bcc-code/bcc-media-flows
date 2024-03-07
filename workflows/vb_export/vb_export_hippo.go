package vb_export

import (
	"github.com/ansel1/merry/v2"
	"github.com/bcc-code/bcc-media-flows/activities"
	"github.com/bcc-code/bcc-media-flows/common"
	"github.com/bcc-code/bcc-media-flows/paths"
	wfutils "github.com/bcc-code/bcc-media-flows/utils/workflows"
	"go.temporal.io/sdk/workflow"
)

var (
	ameFlexResPerformanceWatchFolderInput = paths.Path{
		Drive: paths.IsilonDrive,
		Path:  "system/transcodetemp/hippo/hapqhippo/in",
	}
	ameFlexResPerformanceWatchFolderOutput = paths.Path{
		Drive: paths.IsilonDrive,
		Path:  "system/transcodetemp/hippo/hapqhippo/out",
	}
	ameFlexResQualityWatchFolderInput = paths.Path{
		Drive: paths.IsilonDrive,
		Path:  "system/transcodetemp/hippo/hapalphahippo/in",
	}
	ameFlexResQualityWatchFolderOutput = paths.Path{
		Drive: paths.IsilonDrive,
		Path:  "system/transcodetemp/hippo/hapalphahippo/out",
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

	ctx = workflow.WithActivityOptions(ctx, wfutils.GetDefaultActivityOptions())

	hippoOutputDir := params.TempDir.Append("hippo_output")
	err := wfutils.CreateFolder(ctx, hippoOutputDir)
	if err != nil {
		return nil, err
	}

	if params.AnalyzeResult.FrameRate != 25 && params.AnalyzeResult.FrameRate != 50 && params.AnalyzeResult.FrameRate != 60 {
		return nil, merry.New("Expected 25 or 50 fps input")
	}

	currentVideoFile := params.InputFile
	if params.SubtitleFile != nil {
		// Burn in subtitle
		var videoResult common.VideoResult
		err = wfutils.Execute(ctx, activities.TranscodeToProResActivity, activities.EncodeParams{
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

	var success bool
	inputFolder := ameFlexResPerformanceWatchFolderInput
	outputFile := ameFlexResPerformanceWatchFolderOutput.Append(params.InputFile.Base())
	if params.AnalyzeResult.HasAlpha {
		inputFolder = ameFlexResQualityWatchFolderInput
		outputFile = ameFlexResQualityWatchFolderOutput.Append(params.InputFile.Base())
	}

	err = wfutils.Execute(ctx, activities.CopyFile, activities.MoveFileInput{
		Source:      currentVideoFile,
		Destination: inputFolder.Append(params.InputFile.Base()),
	}).Get(ctx, nil)
	if err != nil {
		return nil, err
	}

	success = false
	err = wfutils.Execute(ctx, activities.WaitForFile, activities.FileInput{
		Path: outputFile,
	}).Get(ctx, &success)
	if err != nil {
		return nil, err
	}
	if !success {
		return nil, merry.New("WaitForFile failed")
	}

	err = wfutils.Execute(ctx, activities.CopyFile, activities.MoveFileInput{
		Source:      outputFile,
		Destination: deliveryFolder.Append("Hippo", params.OriginalFilenameWithoutExt+outputFile.Ext()),
	}).Get(ctx, nil)
	if err != nil {
		return nil, err
	}

	err = wfutils.Execute(ctx, activities.DeletePath, activities.DeletePathInput{
		Path: outputFile,
	}).Get(ctx, nil)
	if err != nil {
		return nil, err
	}

	return &VBExportResult{
		ID: params.ParentParams.VXID,
	}, nil
}
