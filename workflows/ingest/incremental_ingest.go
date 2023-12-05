package ingestworkflows

import (
	"time"

	"github.com/bcc-code/bccm-flows/activities"
	"github.com/bcc-code/bccm-flows/paths"
	wfutils "github.com/bcc-code/bccm-flows/utils/workflows"
	"go.temporal.io/sdk/workflow"
)

type IncrementalParams struct {
	Path string
}

func Incremental(ctx workflow.Context, params IncrementalParams) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting Incremental")

	options := wfutils.GetDefaultActivityOptions()
	ctx = workflow.WithActivityOptions(ctx, options)

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

	err = workflow.Sleep(ctx, time.Minute*2)
	if err != nil {
		return err
	}

	auxDir, err := wfutils.GetWorkflowAuxOutputFolder(ctx)
	if err != nil {
		return err
	}

	rawPathBase := rawPath.Base()
	previewPath := auxDir.Append(rawPathBase[:len(rawPathBase)-len(rawPath.Ext())] + ".mp4")

	livePreviewTask := wfutils.ExecuteWithQueue(ctx, activities.TranscodeLivePreview, activities.TranscodeLivePreviewParams{
		InFilePath:  rawPath,
		OutFilePath: previewPath,
	})

	err = livePreviewTask.Get(ctx, nil)
	if err != nil {
		return err
	}
	err = copyTask.Get(ctx, nil)
	if err != nil {
		return err
	}
	return nil
}
