package vsactivity

import (
	"context"
	"fmt"

	"github.com/bcc-code/bcc-media-flows/paths"
	"github.com/bcc-code/bcc-media-flows/services/vidispine"
	"github.com/bcc-code/bcc-media-platform/backend/asset"

	"go.temporal.io/sdk/activity"
)

type GetExportDataParams struct {
	VXID        string
	Languages   []string
	AudioSource string
	Subclip     string
}

func (a Activities) GetExportDataActivity(ctx context.Context, params GetExportDataParams) (*vidispine.ExportData, error) {
	log := activity.GetLogger(ctx)
	activity.RecordHeartbeat(ctx, "GetExportDataActivity")
	log.Info("Starting GetExportDataActivity")

	client := GetClient()

	audioSource := vidispine.ExportAudioSources.Parse(params.AudioSource)
	if params.AudioSource != "" && audioSource == nil {
		return nil, fmt.Errorf("invalid audioSource: %s", params.AudioSource)
	}

	data, err := vidispine.GetDataForExport(client, params.VXID, params.Languages, audioSource, params.Subclip)
	if err != nil {
		return nil, err
	}

	return data, nil
}

type GetTimedMetadataChaptersParams struct {
	ExportData *vidispine.ExportData
}

func (a Activities) GetTimedMetadataChaptersActivity(ctx context.Context, params GetTimedMetadataChaptersParams) ([]asset.TimedMetadata, error) {
	log := activity.GetLogger(ctx)
	activity.RecordHeartbeat(ctx, "GetTimedMetadataChaptersActivity")
	log.Info("Starting GetTimedMetadataChaptersActivity")

	client := GetClient()

	return vidispine.GetTimedMetadataChapters(client, params.ExportData.Clips)
}

func (a Activities) GetRelatedAudioFiles(ctx context.Context, vxID string) (map[string]paths.Path, error) {
	log := activity.GetLogger(ctx)
	activity.RecordHeartbeat(ctx, "GetRelatedAudioFiles")
	log.Info("Starting GetRelatedAudioFiles")

	client := GetClient()

	audios, err := vidispine.GetRelatedAudioPaths(client, vxID)

	if err != nil {
		return nil, err
	}

	var result = map[string]paths.Path{}
	for lang, p := range audios {
		result[lang], err = paths.Parse(p)
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}
