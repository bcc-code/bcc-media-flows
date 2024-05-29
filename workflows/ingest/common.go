package ingestworkflows

import (
	"strconv"

	"github.com/bcc-code/bcc-media-flows/services/emails"
	"github.com/bcc-code/bcc-media-flows/services/telegram"
	miscworkflows "github.com/bcc-code/bcc-media-flows/workflows/misc"

	"github.com/ansel1/merry/v2"
	"github.com/bcc-code/bcc-media-flows/activities"
	vsactivity "github.com/bcc-code/bcc-media-flows/activities/vidispine"
	"github.com/bcc-code/bcc-media-flows/paths"
	"github.com/bcc-code/bcc-media-flows/services/ingest"
	"github.com/bcc-code/bcc-media-flows/services/notifications"
	wfutils "github.com/bcc-code/bcc-media-flows/utils/workflows"
	"github.com/samber/lo"
	"go.temporal.io/api/enums/v1"
	"go.temporal.io/sdk/workflow"
)

type ImportTagResult struct {
	AssetID     string
	ImportJobID string
}

func ImportFileAsTag(ctx workflow.Context, tag string, path paths.Path, title string) (*ImportTagResult, error) {
	var result vsactivity.CreatePlaceholderResult
	err := wfutils.Execute(ctx, activities.Vidispine.CreatePlaceholderActivity, vsactivity.CreatePlaceholderParams{
		Title: title,
	}).Get(ctx, &result)
	if err != nil {
		return nil, err
	}
	var job vsactivity.JobResult
	err = wfutils.Execute(ctx, activities.Vidispine.ImportFileAsShapeActivity, vsactivity.ImportFileAsShapeParams{
		AssetID:  result.AssetID,
		FilePath: path,
		ShapeTag: tag,
	}).Get(ctx, &job)
	if err != nil {
		return nil, err
	}
	return &ImportTagResult{
		AssetID:     result.AssetID,
		ImportJobID: job.JobID,
	}, nil
}

func CreatePreviews(ctx workflow.Context, assetIDs []string) error {
	wfFutures := createPreviewsAsync(ctx, assetIDs)

	for _, f := range wfFutures {
		err := f.Get(ctx, nil)
		if err != nil {
			return err
		}
	}

	return nil
}

func createPreviewsAsync(ctx workflow.Context, assetIDs []string) []workflow.ChildWorkflowFuture {
	var wfFutures []workflow.ChildWorkflowFuture
	opts := workflow.GetChildWorkflowOptions(ctx)
	opts.ParentClosePolicy = enums.PARENT_CLOSE_POLICY_ABANDON
	ctx = workflow.WithChildOptions(ctx, opts)
	for _, id := range assetIDs {
		wfFutures = append(wfFutures, workflow.ExecuteChildWorkflow(ctx, miscworkflows.TranscodePreviewVX, miscworkflows.TranscodePreviewVXInput{
			VXID: id,
		}))
	}

	return wfFutures
}

func transcribe(ctx workflow.Context, assetIDs []string, language string) error {
	var wfFutures []workflow.ChildWorkflowFuture
	opts := workflow.GetChildWorkflowOptions(ctx)
	opts.ParentClosePolicy = enums.PARENT_CLOSE_POLICY_ABANDON
	for _, id := range assetIDs {
		wfFutures = append(wfFutures, workflow.ExecuteChildWorkflow(ctx, miscworkflows.TranscribeVX, miscworkflows.TranscribeVXInput{
			VXID:     id,
			Language: language,
		}))
	}

	for _, f := range wfFutures {
		err := f.Get(ctx, nil)
		if err != nil {
			return err
		}
	}
	return nil
}

func getOrderFormFilename(orderForm OrderForm, file paths.Path, props ingest.JobProperty) (string, error) {
	switch orderForm {
	case OrderFormRawMaterial, OrderFormMusic, OrderFormUpload:
		// return filename without extension
		base := file.Base()
		ext := file.Ext()
		return base[:len(base)-len(ext)], nil
	case OrderFormLEDMaterial, OrderFormVBMaster, OrderFormSeriesMaster, OrderFormOtherMaster:
		return masterFilename(props)
	}
	return "", merry.New("Unsupported order form")
}

func notifyImportCompleted(ctx workflow.Context, recipients []string, jobID int, filesByAssetID map[string]paths.Path) error {
	var content notifications.ImportCompleted
	err := workflow.SideEffect(ctx, func(ctx workflow.Context) any {
		return notifications.ImportCompleted{
			Title: "Import completed",
			JobID: strconv.Itoa(jobID),
			Files: lo.Map(lo.Entries(filesByAssetID), func(entry lo.Entry[string, paths.Path], _ int) notifications.File {
				return notifications.File{
					VXID: entry.Key,
					Name: entry.Value.Base(),
				}
			}),
		}
	}).Get(content)

	if err != nil {
		return err
	}

	msg, _ := telegram.NewMessage(telegram.ChatOther, content)
	wfutils.Execute(ctx, activities.Util.SendTelegramMessage, msg).Get(ctx, nil)

	email, _ := emails.NewMessage(content, recipients, nil, nil)
	return wfutils.Execute(ctx, activities.Util.SendEmail, email).Get(ctx, nil)
}

func notifyImportFailed(ctx workflow.Context, recipients []string, jobID int, filesByAssetID []paths.Path, importError error) error {
	content := notifications.ImportFailed{
		Error: importError.Error(),
		Title: "Import failed",
		JobID: strconv.Itoa(jobID),
		Files: lo.Map(filesByAssetID, func(entry paths.Path, _ int) notifications.File {
			return notifications.File{
				Name: entry.Base(),
			}
		}),
	}

	msg, _ := telegram.NewMessage(telegram.ChatOther, content)
	wfutils.Execute(ctx, activities.Util.SendTelegramMessage, msg).Get(ctx, nil)

	email, _ := emails.NewMessage(content, recipients, nil, nil)
	return wfutils.Execute(ctx, activities.Util.SendEmail, email).Get(ctx, nil)
}
