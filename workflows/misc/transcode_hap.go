package miscworkflows

import (
	"github.com/bcc-code/bcc-media-flows/activities"
	"github.com/bcc-code/bcc-media-flows/paths"
	wfutils "github.com/bcc-code/bcc-media-flows/utils/workflows"

	"go.temporal.io/sdk/workflow"
)

type TranscodeHAPInput struct {
	FilePath  string
	OutputDir string
}

func TranscodeHAP(
	ctx workflow.Context,
	params TranscodeHAPInput,
) (*activities.HAPResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting TranscodeHAP")

	ctx = workflow.WithActivityOptions(ctx, wfutils.GetDefaultActivityOptions())

	filePath := paths.MustParse(params.FilePath)
	outputDir := paths.MustParse(params.OutputDir)

	return wfutils.Execute(ctx, activities.Video.TranscodeToHAPActivity, activities.HAPInput{
		FilePath:  filePath,
		OutputDir: outputDir,
	}).Result(ctx)
}