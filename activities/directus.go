package activities

import (
	"context"
	"fmt"

	"github.com/bcc-code/bcc-media-flows/directus"
)

var Directus *DirectusActivities

type DirectusActivities struct {
	Client *directus.Client
}

type GetAssetByMediabankenIDInput struct {
	MediabankenID string
}

type GetAssetByMediabankenIDResult struct {
	Asset *directus.Asset
}

type CreateMediaItemInput struct {
	Label           string
	Type            string
	AssetID         int64
	Title           string
	ParentEpisodeID *int
	ParentStartsAt  *int64
	ParentEndsAt    *int64
	StyledImageID   string
}

type CreateMediaItemResult struct {
	MediaItem *directus.MediaItem
}

type CreateShortInput struct {
	MediaItemID string
	Status      string
}

type CreateShortResult struct {
	Short *directus.Short
}

type CheckDirectusAssetExistsInput struct {
	MediabankenID string
}

type CheckDirectusAssetExistsResult struct {
	Exists bool
}

type UploadFileInput struct {
	FilePath string
}

type UploadFileResult struct {
	FileID string
}

type CreateStyledImageInput struct {
	ImageID string
	Style   string
}

func (a *DirectusActivities) CheckDirectusAssetExists(ctx context.Context, input CheckDirectusAssetExistsInput) (bool, error) {
	return a.Client.AssetExists(input.MediabankenID)
}

func (a *DirectusActivities) UploadFile(ctx context.Context, input UploadFileInput) (*directus.File, error) {
	return a.Client.UploadFile(input.FilePath)
}

// CreateStyledImage creates a styled image in Directus
func (a *DirectusActivities) CreateStyledImage(ctx context.Context, input CreateStyledImageInput) (*directus.StyledImage, error) {
	return a.Client.CreateStyledImage(input.ImageID, input.Style)
}

// GetAssetByMediabankenID retrieves an asset by its mediabanken_id
func (a *DirectusActivities) GetAssetByMediabankenID(ctx context.Context, input GetAssetByMediabankenIDInput) (*GetAssetByMediabankenIDResult, error) {
	asset, err := a.Client.GetAssetByMediabankenID(input.MediabankenID)
	if err != nil {
		return nil, err
	}
	return &GetAssetByMediabankenIDResult{Asset: asset}, nil
}

// CreateMediaItem creates a new media item in Directus
func (a *DirectusActivities) CreateMediaItem(ctx context.Context, input CreateMediaItemInput) (*CreateMediaItemResult, error) {
	mediaItem, err := a.Client.CreateMediaItem(directus.MediaItemCreate{
		Label:           input.Label,
		Type:            input.Type,
		AssetID:         input.AssetID,
		Title:           input.Title,
		ParentEpisodeID: input.ParentEpisodeID,
		ParentStartsAt:  input.ParentStartsAt,
		ParentEndsAt:    input.ParentEndsAt,
		Images: directus.MediaItemStyledImageCRUD{
			Create: []directus.MediaItemStyledImageRelation{
				{
					MediaItemsID:   "+",
					StyledImagesID: input.StyledImageID,
				},
			},
			Update: []directus.MediaItemStyledImageRelation{},
			Delete: []directus.MediaItemStyledImageRelation{},
		},
	})

	if err != nil {
		return nil, fmt.Errorf("failed to create media item: %w", err)
	}

	return &CreateMediaItemResult{MediaItem: mediaItem}, nil
}

// CreateShort creates a new short in Directus
func (a *DirectusActivities) CreateShort(ctx context.Context, input CreateShortInput) (*CreateShortResult, error) {
	short, err := a.Client.CreateShort(directus.ShortCreate{
		MediaItemID: input.MediaItemID,
		Status:      input.Status,
		Roles:       []string{"bcc-members"},
	})

	if err != nil {
		return nil, fmt.Errorf("failed to create short: %w", err)
	}

	return &CreateShortResult{Short: short}, nil
}
