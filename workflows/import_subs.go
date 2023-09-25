package workflows

import (
	"fmt"
	"strings"
	"time"

	"github.com/bcc-code/bccm-flows/activities"
	"github.com/bcc-code/bccm-flows/activities/vidispine"
	"github.com/bcc-code/bccm-flows/utils"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

type ImportSubtitlesFromSubtransInput struct {
	VXId string
}

func ImportSubtitlesFromSubtrans(
	ctx workflow.Context,
	params ImportSubtitlesFromSubtransInput,
) error {
	logger := workflow.GetLogger(ctx)

	options := workflow.ActivityOptions{
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:        time.Minute * 3,
			MaximumAttempts:        10,
			MaximumInterval:        time.Hour * 1,
			NonRetryableErrorTypes: []string{},
		},
		StartToCloseTimeout:    time.Hour * 4,
		ScheduleToCloseTimeout: time.Hour * 48,
		HeartbeatTimeout:       time.Minute * 5,
		TaskQueue:              utils.GetQueue(),
	}

	ctx = workflow.WithActivityOptions(ctx, options)

	logger.Info("Starting sub import flow")

	input := activities.GetSubtransIDInput{
		VXID:     params.VXId,
		NoSubsOK: true,
	}

	subtransIDRespone := &activities.GetSubtransIDOutput{}
	err := workflow.ExecuteActivity(ctx, activities.GetSubtransIDActivity, input).Get(ctx, subtransIDRespone)
	if err != nil {
		return err
	}

	outputPath, _ := getWorkflowOutputFolder(ctx)

	subsList := map[string]string{}
	err = workflow.ExecuteActivity(ctx, activities.GetSubtitlesActivity, activities.GetSubtitlesInput{
		SubtransID:        subtransIDRespone.SubtransID,
		Format:            "srt",
		ApprovedOnly:      false,
		DestinationFolder: outputPath,
		//FilePrefix:        "subs_", <-- Generated by subtrans if empty
	}).Get(ctx, &subsList)
	if err != nil {
		return err
	}

	activities := []workflow.Future{}
	for lang, sub := range subsList {
		lang = strings.ToLower(lang)

		act := workflow.ExecuteActivity(ctx, vidispine.ImportFileAsSidecarActivity, vidispine.ImportSubtitleAsSidecarParams{
			AssetID:  params.VXId,
			Language: lang,
			FilePath: sub,
		})

		workflow.ExecuteActivity(ctx, vidispine.ImportFileAsShapeActivity, vidispine.ImportFileAsShapeParams{
			AssetID:  params.VXId,
			FilePath: sub,
			ShapeTag: fmt.Sprintf("sub_%s_%s", lang, "srt"),
		})

		activities = append(activities, act)
	}

	for _, act := range activities {
		err := act.Get(ctx, nil)
		if err != nil {
			return err
		}
	}

	return nil
}
