package workflows

import (
	"github.com/bcc-code/bcc-media-flows/activities"
	"github.com/bcc-code/bcc-media-flows/paths"
	wfutils "github.com/bcc-code/bcc-media-flows/utils/workflows"

	"go.temporal.io/sdk/workflow"
)

// TranscodePreviewFileInput is the input to the TranscodePreviewFile workflow
type TranscodePreviewFileInput struct {
	FilePath string
}

// TranscodePreviewFile can be used to test the transcode activity locally where you have no access to vidispine
// or would like to avoid writing to vidispine. Output folder will be set to the same as where the file is originated.
func TranscodePreviewFile(
	ctx workflow.Context,
	params TranscodePreviewFileInput,
) error {

	logger := workflow.GetLogger(ctx)
	options := wfutils.GetDefaultActivityOptions()

	ctx = workflow.WithActivityOptions(ctx, options)

	logger.Info("Starting TranscodePreviewFile")

	filePath, err := paths.Parse(params.FilePath)
	if err != nil {
		return err
	}

	previewResponse := &activities.TranscodePreviewResponse{}
	err = wfutils.ExecuteWithQueue(ctx, activities.TranscodePreview, activities.TranscodePreviewParams{
		FilePath:           filePath,
		DestinationDirPath: filePath.Dir(),
	}).Get(ctx, previewResponse)

	if err != nil {
		return err
	}

	return err
}
