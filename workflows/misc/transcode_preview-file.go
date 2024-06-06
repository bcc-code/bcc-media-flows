package miscworkflows

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
	logger.Info("Starting TranscodePreviewFile")

	ctx = workflow.WithActivityOptions(ctx, wfutils.GetDefaultActivityOptions())

	filePath := paths.MustParse(params.FilePath)

	previewResponse := &activities.TranscodePreviewResponse{}
	err := wfutils.Execute(ctx, activities.Video.TranscodePreview, activities.TranscodePreviewParams{
		FilePath:           filePath,
		DestinationDirPath: filePath.Dir(),
	}).Get(ctx, previewResponse)

	if err != nil {
		return err
	}

	return err
}
