package activities

import (
	"context"
	"fmt"
	"github.com/bcc-code/bcc-media-flows/services/directus"
)

var Directus *DirectusActivities

type DirectusActivities struct {
	Client         *directus.Client
	ShortsFolderID string
}

type CreateTagInput struct {
	Code string
	Name string
}

type CreateMediaItemTagInput struct {
	MediaItemID string
	TagID       string
}

type CreateMediaItemInput struct {
	Label           string
	Type            string
	AssetID         string
	Title           string
	ParentEpisodeID string
	ParentStartsAt  *int64
	ParentEndsAt    *int64
	StyledImageID   string
}

type CreateShortInput struct {
	MediaItemID string
	Status      string
}

type CreateStyledImageInput struct {
	ImageID string
	Style   string
}

type GetOrCreateTagInput struct {
	Code string
	Name string
}

type UploadFileInput struct {
	File             string
	DirectusFolderID string
}

func (a *DirectusActivities) CheckDirectusAssetExists(ctx context.Context, mediabankenID string) (bool, error) {
	return a.Client.AssetExists(mediabankenID)
}

func (a *DirectusActivities) UploadFile(ctx context.Context, args UploadFileInput) (*directus.File, error) {
	return a.Client.UploadFile(args.DirectusFolderID, args.File)
}

func (a *DirectusActivities) CreateStyledImage(ctx context.Context, input CreateStyledImageInput) (*directus.StyledImage, error) {
	return a.Client.CreateStyledImage(input.ImageID, input.Style)
}

func (a *DirectusActivities) GetAssetByMediabankenID(ctx context.Context, mediabankenID string) (*directus.Asset, error) {
	asset, err := a.Client.GetAssetByMediabankenID(mediabankenID)
	if err != nil {
		return nil, err
	}
	return asset, nil
}

func (a *DirectusActivities) CreateMediaItem(ctx context.Context, input CreateMediaItemInput) (*directus.MediaItem, error) {
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

	return mediaItem, nil
}

func (a *DirectusActivities) GetTagByCode(ctx context.Context, input string) (*directus.Tag, error) {
	tag, err := a.Client.GetTagByCode(input)
	if err != nil {
		return nil, fmt.Errorf("failed to get tag by code: %w", err)
	}
	return tag, nil
}

// CreateMediaItemTag creates a relationship between a media item and a tag
func (a *DirectusActivities) CreateMediaItemTag(ctx context.Context, input CreateMediaItemTagInput) (*directus.MediaItemTag, error) {
	mediaItemTag, err := a.Client.CreateMediaItemTag(input.MediaItemID, input.TagID)
	if err != nil {
		return nil, fmt.Errorf("failed to create media item tag: %w", err)
	}
	return mediaItemTag, nil
}

func (a *DirectusActivities) CreateTag(ctx context.Context, input CreateTagInput) (*directus.Tag, error) {
	tag, err := a.Client.CreateTag(input.Code, input.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to create tag: %w", err)
	}
	return tag, nil
}

func (a *DirectusActivities) GetOrCreateTag(ctx context.Context, input GetOrCreateTagInput) (*directus.Tag, error) {
	// Try to get the tag by code
	tagResult, err := a.GetTagByCode(ctx, input.Code)
	if err == nil && tagResult != nil {
		return tagResult, nil
	}

	// Create tag if not found
	name := input.Name
	if name == "" {
		name = input.Code
	}
	createTagResult, err := a.CreateTag(ctx, CreateTagInput{Code: input.Code, Name: name})
	if err != nil {
		return nil, fmt.Errorf("failed to create tag: %w", err)
	}
	return createTagResult, nil
}

func (a *DirectusActivities) CreateShort(ctx context.Context, input CreateShortInput) (*directus.Short, error) {
	short, err := a.Client.CreateShort(directus.ShortCreate{
		MediaItemID: input.MediaItemID,
		Status:      input.Status,
		Roles:       []string{"bcc-members"},
	})

	if err != nil {
		return nil, fmt.Errorf("failed to create short: %w", err)
	}

	return short, nil
}
