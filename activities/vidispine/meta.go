package vsactivity

import (
	"context"
	"fmt"
	"strings"

	bccmflows "github.com/bcc-code/bcc-media-flows"
	"github.com/bcc-code/bcc-media-flows/paths"
	"github.com/bcc-code/bcc-media-flows/services/vidispine/vsapi"
	"github.com/bcc-code/bcc-media-flows/services/vidispine/vscommon"
	"go.temporal.io/sdk/activity"
)

type VXOnlyParam struct {
	VXID string
}

type GetFileFromVXParams struct {
	VXID string
	Tags []string
}

type GetFileFromVXResult struct {
	FilePath paths.Path
	ShapeTag string
}

func (a Activities) GetFileFromVXActivity(ctx context.Context, params GetFileFromVXParams) (*GetFileFromVXResult, error) {
	log := activity.GetLogger(ctx)
	log.Info("Starting GetFileFromVXActivity")

	vsClient := GetClient()

	shapes, err := vsClient.GetShapes(params.VXID)
	if err != nil {
		return nil, err
	}

	for _, tag := range params.Tags {
		shape := shapes.GetShape(tag)
		if shape == nil {
			log.Debug("No shape found for tag: %s", tag)
			continue
		}

		return &GetFileFromVXResult{
			FilePath: paths.MustParse(shape.GetPath()),
			ShapeTag: tag,
		}, nil
	}

	return nil, fmt.Errorf("no shape found for tags: %v", params.Tags)
}

func (a Activities) GetVXMetadata(_ context.Context, params VXOnlyParam) (*vsapi.MetadataResult, error) {
	vsClient := GetClient()
	return vsClient.GetMetadata(params.VXID)
}

type VXMetadataFieldParams = vsapi.ItemMetadataFieldParams

type SetVXMetadataFieldResult struct {
}

func (a Activities) SetVXMetadataFieldActivity(ctx context.Context, params vsapi.ItemMetadataFieldParams) (*SetVXMetadataFieldResult, error) {
	log := activity.GetLogger(ctx)
	log.Info("Starting SetVXMetadataFieldActivity")

	vsClient := GetClient()

	err := vsClient.SetItemMetadataField(params)
	return nil, err
}

func (a Activities) AddToVXMetadataFieldActivity(ctx context.Context, params vsapi.ItemMetadataFieldParams) (*SetVXMetadataFieldResult, error) {
	log := activity.GetLogger(ctx)
	log.Info("Starting SetVXMetadataFieldActivity")

	vsClient := GetClient()

	err := vsClient.AddToItemMetadataField(params)
	return nil, err
}

type GetResolutionsParams struct {
	VXID string
}

func GetResolutions(ctx context.Context, params GetResolutionsParams) ([]vsapi.Resolution, error) {
	log := activity.GetLogger(ctx)
	log.Info("Starting GetResolutions")

	vsClient := GetClient()

	return vsClient.GetResolutions(params.VXID)
}

func (a Activities) GetRelations(ctx context.Context, assetID string) ([]vsapi.Relation, error) {
	log := activity.GetLogger(ctx)
	log.Info("Starting GetRelations")

	vsClient := GetClient()

	return vsClient.GetRelations(assetID)
}

// UpdateAssetRelations attempts to find languages of related audio files and update the metadata
// of this asset with the link
func (a Activities) UpdateAssetRelations(ctx context.Context, params VXOnlyParam) ([]string, error) {
	vxID := params.VXID
	log := activity.GetLogger(ctx)
	log.Info("Starting UpdateAssetRelations")

	vsClient := GetClient()

	relations, err := vsClient.GetRelations(vxID)
	if err != nil {
		return nil, err
	}

	updatedLanguages := []string{}
	for _, relation := range relations {
		other := relation.Direction.Source
		if other == vxID {
			other = relation.Direction.Target
		}

		meta, err := vsClient.GetMetadata(other)
		if err != nil {
			return nil, err
		}

		if meta.Get(vscommon.FieldOriginalAudioCodec, "None") == "None" {
			continue
		}

		// Drop the extension, if it exists
		title := meta.Get(vscommon.FieldTitle, "No title found")
		titleSplit := strings.Split(title, ".")
		if len(titleSplit) > 1 {
			titleSplit = titleSplit[:len(titleSplit)-1]
		}
		title = strings.Join(titleSplit, ".")

		if title[len(title)-4] != '_' && title[len(title)-4] != '-' {
			// If the foruth to last character is not an underscore or dash, it is not a language code
			continue
		}

		// Get the last three characters of the title, as it should be a language code
		title = title[len(title)-3:]

		if l, ok := bccmflows.LanguagesByISO[strings.ToLower(title)]; ok {
			err := vsClient.SetItemMetadataField(
				vsapi.ItemMetadataFieldParams{
					ItemID:  vxID,
					GroupID: "System",
					Key:     l.RelatedMBFieldID,
					Value:   other,
				})
			if err != nil {
				return nil, err
			}
			updatedLanguages = append(updatedLanguages, l.ISO6391)
		}
	}

	return updatedLanguages, nil
}

func (a Activities) GetShapes(ctx context.Context, params VXOnlyParam) (*vsapi.ShapeResult, error) {
	log := activity.GetLogger(ctx)
	log.Info("Starting GetShapes")

	vsClient := GetClient()

	return vsClient.GetShapes(params.VXID)
}
