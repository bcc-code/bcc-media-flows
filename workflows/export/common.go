package export

import (
	"fmt"
	"github.com/bcc-code/bcc-media-flows/services/telegram"

	wfutils "github.com/bcc-code/bcc-media-flows/utils/workflows"
	"go.temporal.io/sdk/workflow"
)

func notifyExportDone(ctx workflow.Context, chat telegram.Chat, params VXExportChildWorkflowParams, flow string, notificationEmoji rune) {
	message := fmt.Sprintf("%c Export of `%s` finished.\nDestination: `%s`", notificationEmoji, params.ExportData.Title, flow)

	wfutils.SendTelegramText(
		ctx,
		chat,
		message,
	)
}
