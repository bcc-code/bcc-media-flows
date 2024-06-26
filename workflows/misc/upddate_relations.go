package miscworkflows

import (
	"fmt"
	"strings"

	"github.com/bcc-code/bcc-media-flows/services/telegram"

	"github.com/bcc-code/bcc-media-flows/activities"
	vsactivity "github.com/bcc-code/bcc-media-flows/activities/vidispine"
	wfutils "github.com/bcc-code/bcc-media-flows/utils/workflows"
	"go.temporal.io/sdk/workflow"
)

type UpdateAssetRelationsParams struct {
	AssetID string
}

func UpdateAssetRelations(ctx workflow.Context, params UpdateAssetRelationsParams) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting UpdateAssetRelations")

	opts := wfutils.GetDefaultActivityOptions()
	ctx = workflow.WithActivityOptions(ctx, opts)

	updatedLangs, err := wfutils.Execute(ctx, activities.Vidispine.UpdateAssetRelations, vsactivity.VXOnlyParam{
		VXID: params.AssetID,
	}).Result(ctx)

	if err != nil {
		wfutils.SendTelegramText(ctx, telegram.ChatOther, fmt.Sprintf("🟥 Failed to update asset relations: ```%v```", err))
		return err
	}

	wfutils.SendTelegramText(ctx,
		telegram.ChatOther,
		fmt.Sprintf(
			"🟩 Updated asset relations for asset %s with for %d languages: %s",
			params.AssetID,
			len(updatedLangs),
			strings.Join(updatedLangs, ", "),
		),
	)

	return nil
}
