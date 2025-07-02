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

type GetTagByCodeInput struct {
	Code string
}

type GetTagByCodeResult struct {
	Tag *directus.Tag
}

type CreateTagInput struct {
	Code string
	Name string
}

type CreateTagResult struct {
	Tag *directus.Tag
}

type CreateMediaItemTagInput struct {
	MediaItemID string
	TagID       string
}

type CreateMediaItemTagResult struct {
	MediaItemTag *directus.MediaItemTag
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

type GetOrCreateTagInput struct {
	Code string
	Name string // optional, fallback to Code
}

type GetOrCreateTagResult struct {
	Tag *directus.Tag
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

// GetTagByCode finds a tag by its code
func (a *DirectusActivities) GetTagByCode(ctx context.Context, input GetTagByCodeInput) (*GetTagByCodeResult, error) {
	tag, err := a.Client.GetTagByCode(input.Code)
	if err != nil {
		return nil, fmt.Errorf("failed to get tag by code: %w", err)
	}
	return &GetTagByCodeResult{Tag: tag}, nil
}

// CreateMediaItemTag creates a relationship between a media item and a tag
func (a *DirectusActivities) CreateMediaItemTag(ctx context.Context, input CreateMediaItemTagInput) (*CreateMediaItemTagResult, error) {
	mediaItemTag, err := a.Client.CreateMediaItemTag(input.MediaItemID, input.TagID)
	if err != nil {
		return nil, fmt.Errorf("failed to create media item tag: %w", err)
	}
	return &CreateMediaItemTagResult{MediaItemTag: mediaItemTag}, nil
}

// CreateTag creates a new tag in Directus
func (a *DirectusActivities) CreateTag(ctx context.Context, input CreateTagInput) (*CreateTagResult, error) {
	tag, err := a.Client.CreateTag(input.Code, input.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to create tag: %w", err)
	}
	return &CreateTagResult{Tag: tag}, nil
}

// GetOrCreateTag checks if a tag exists with the given code, creates it if it doesn't exist, and returns its ID
func (a *DirectusActivities) GetOrCreateTag(ctx context.Context, input GetOrCreateTagInput) (*GetOrCreateTagResult, error) {
	// Try to get the tag by code
	tagResult, err := a.GetTagByCode(ctx, GetTagByCodeInput{Code: input.Code})
	if err == nil && tagResult.Tag != nil {
		return &GetOrCreateTagResult{Tag: tagResult.Tag}, nil
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
	return &GetOrCreateTagResult{Tag: createTagResult.Tag}, nil
}

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
