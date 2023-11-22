package vidispine

import (
	"context"
	"fmt"

	"github.com/bcc-code/bcc-media-platform/backend/asset"

	"github.com/bcc-code/bccm-flows/services/vidispine"
	"go.temporal.io/sdk/activity"
)

type GetExportDataParams struct {
	VXID        string
	Languages   []string
	AudioSource string
	Subclip     string
}

func GetExportDataActivity(ctx context.Context, params *GetExportDataParams) (*vidispine.ExportData, error) {
	log := activity.GetLogger(ctx)
	activity.RecordHeartbeat(ctx, "GetExportDataActivity")
	log.Info("Starting GetExportDataActivity")

	client := GetClient()

	audioSource := vidispine.ExportAudioSources.Parse(params.AudioSource)
	if params.AudioSource != "" && audioSource == nil {
		return nil, fmt.Errorf("invalid audioSource: %s", params.AudioSource)
	}

	data, err := client.GetDataForExport(params.VXID, params.Languages, audioSource, params.Subclip)
	if err != nil {
		return nil, err
	}

	return data, nil
}

type GetChapterDataParams struct {
	ExportData *vidispine.ExportData
}

func GetChapterDataActivity(ctx context.Context, params *GetChapterDataParams) ([]asset.Chapter, error) {
	log := activity.GetLogger(ctx)
	activity.RecordHeartbeat(ctx, "GetChapterDataActivity")
	log.Info("Starting GetChapterDataActivity")

	client := GetClient()

	return client.GetChapterData(params.ExportData)
}
