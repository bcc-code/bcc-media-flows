package directus

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/go-resty/resty/v2"
)

type Client struct {
	BaseURL string
	APIKey  string
	client  *resty.Client
}

func NewClient(baseURL, apiKey string) *Client {
	client := resty.New()
	client.SetHeader("Authorization", "Bearer "+apiKey)
	return &Client{
		BaseURL: baseURL,
		APIKey:  apiKey,
		client:  client,
	}
}

type directusResponse struct {
	Data []interface{} `json:"data"`
}

type Asset struct {
	ID            int64  `json:"id"`
	MediabankenID string `json:"mediabanken_id"`
}

type File struct {
	ID               string   `json:"id"`
	Storage          string   `json:"storage"`
	FilenameDisk     string   `json:"filename_disk"`
	FilenameDownload string   `json:"filename_download"`
	Title            string   `json:"title"`
	Type             string   `json:"type"`
	Folder           *string  `json:"folder"`
	CreatedOn        string   `json:"created_on"`
	UploadedBy       string   `json:"uploaded_by"`
	UploadedOn       string   `json:"uploaded_on"`
	ModifiedBy       *string  `json:"modified_by"`
	ModifiedOn       string   `json:"modified_on"`
	Filesize         string   `json:"filesize"`
	Width            *int     `json:"width"`
	Height           *int     `json:"height"`
	FocalPointX      *float64 `json:"focal_point_x"`
	FocalPointY      *float64 `json:"focal_point_y"`
	Duration         *int     `json:"duration"`
	Description      *string  `json:"description"`
	Location         *string  `json:"location"`
	Tags             []string `json:"tags"`
}

// StyledImage represents a styled image in Directus
type StyledImage struct {
	ID          string     `json:"id"`
	Style       string     `json:"style"`
	Language    string     `json:"language"`
	File        string     `json:"file"`
	DateCreated *time.Time `json:"date_created,omitempty"`
	DateUpdated *time.Time `json:"date_updated,omitempty"`
	UserCreated *string    `json:"user_created,omitempty"`
	UserUpdated *string    `json:"user_updated,omitempty"`
}

// Short represents a short in Directus
type Short struct {
	ID          string `json:"id"`
	MediaItemID string `json:"mediaitem_id"`
	Status      string `json:"status"`
}

// ShortCreate is used when creating a new short
type ShortCreate struct {
	MediaItemID string   `json:"mediaitem_id"`
	Status      string   `json:"status"`
	Roles       []string `json:"roles,omitempty"`
}

// MediaItem represents a media item in Directus"
type MediaItem struct {
	ID              string `json:"id"`
	Label           string `json:"label"`
	Type            string `json:"type"`
	AssetID         int64  `json:"asset_id"`
	Title           string `json:"title"`
	ParentEpisodeID *int   `json:"parent_episode_id"`
	ParentStartsAt  *int64 `json:"parent_starts_at"`
	ParentEndsAt    *int64 `json:"parent_ends_at"`
}

// MediaItemCreate is used when creating a new media item
type MediaItemCreate struct {
	Label           string                   `json:"label"`
	Type            string                   `json:"type"`
	AssetID         int64                    `json:"asset_id"`
	Title           string                   `json:"title"`
	ParentEpisodeID *int                     `json:"parent_episode_id"`
	ParentStartsAt  *int64                   `json:"parent_starts_at"`
	ParentEndsAt    *int64                   `json:"parent_ends_at"`
	Images          MediaItemStyledImageCRUD `json:"images"`
}

// MediaItemStyledImageRelation represents the relationship between a media item and a styled image
type MediaItemStyledImageRelation struct {
	MediaItemsID   string `json:"mediaitems_id"`
	StyledImagesID string `json:"styledimages_id"`
}

// MediaItemStyledImageCRUD represents CRUD operations for media item styled image relations
type MediaItemStyledImageCRUD struct {
	Create []MediaItemStyledImageRelation `json:"create"`
	Update []MediaItemStyledImageRelation `json:"update"`
	Delete []MediaItemStyledImageRelation `json:"delete"`
}

type StyledImageCreate struct {
	Style    string `json:"style"`
	Language string `json:"language"`
	File     string `json:"file"`
}

// GetAssetByMediabankenID retrieves an asset by its mediabanken_id
func (c *Client) GetAssetByMediabankenID(mediabankenID string) (*Asset, error) {
	endpoint := fmt.Sprintf("%s/items/assets", c.BaseURL)
	result := &struct {
		Data []Asset `json:"data"`
	}{}

	resp, err := c.client.R().
		SetQueryParams(map[string]string{
			"filter[mediabanken_id][_eq]": mediabankenID,
			"limit":                       "1",
		}).
		SetResult(result).
		Get(endpoint)

	if err != nil {
		return nil, fmt.Errorf("failed to fetch asset: %w", err)
	}

	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("Directus API error: %s", resp.Status())
	}

	if len(result.Data) == 0 {
		return nil, nil
	}

	return &result.Data[0], nil
}

// AssetExists checks if an asset with the given mediabanken_id exists in Directus
func (c *Client) AssetExists(mediabankenID string) (bool, error) {
	endpoint := fmt.Sprintf("%s/items/assets", c.BaseURL)
	result := &directusResponse{}
	resp, err := c.client.R().
		SetQueryParams(map[string]string{
			"filter[mediabanken_id][_eq]": mediabankenID,
			"limit":                       "1",
		}).
		SetResult(result).
		Get(endpoint)
	if err != nil {
		return false, err
	}
	if resp.StatusCode() != 200 {
		return false, fmt.Errorf("Directus API error: %s", resp.Status())
	}
	return len(result.Data) > 0, nil
}

// CreateStyledImage creates a styled image in Directus and returns the created styled image
func (c *Client) CreateStyledImage(imageID, style string) (*StyledImage, error) {
	if imageID == "" {
		return nil, fmt.Errorf("imageID is required")
	}

	validStyles := []string{"poster", "default", "icon", "album", "featured"}
	valid := false
	for _, s := range validStyles {
		if s == style {
			valid = true
			break
		}
	}

	if !valid {
		return nil, fmt.Errorf("invalid style: %s. Valid styles: %v", style, validStyles)
	}

	result := &struct {
		Data StyledImage `json:"data"`
	}{}

	resp, err := c.client.R().
		SetResult(result).
		SetBody(StyledImageCreate{
			Style:    style,
			Language: "no",
			File:     imageID,
		}).
		Post(fmt.Sprintf("%s/items/styledimages", c.BaseURL))

	if err != nil {
		return nil, fmt.Errorf("failed to create styled image: %w", err)
	}

	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("Directus API error: %s", resp.Status())
	}

	if result.Data.ID == "" {
		return nil, fmt.Errorf("invalid response from Directus: missing styled image ID")
	}

	return &result.Data, nil
}

// CreateShort creates a new short in Directus
func (c *Client) CreateShort(short ShortCreate) (*Short, error) {
	endpoint := fmt.Sprintf("%s/items/shorts", c.BaseURL)
	result := &struct {
		Data Short `json:"data"`
	}{}

	// Create the short with roles in the format Directus expects
	shortData := map[string]interface{}{
		"mediaitem_id": short.MediaItemID,
		"status":       short.Status,
	}

	// If there are roles, include them in the initial creation
	if len(short.Roles) > 0 {
		// Format roles as a relationship update
		rolesUpdate := make([]map[string]interface{}, len(short.Roles))
		for i, role := range short.Roles {
			rolesUpdate[i] = map[string]interface{}{
				"usergroups_code": role,
			}
		}

		shortData["roles"] = map[string]interface{}{
			"create": rolesUpdate,
			"update": []interface{}{},
			"delete": []interface{}{},
		}
	}

	resp, err := c.client.R().
		SetResult(result).
		SetBody(shortData).
		Post(endpoint)

	if err != nil {
		return nil, fmt.Errorf("failed to create short: %w", err)
	}

	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("Directus API error: %s - %s", resp.Status(), resp.String())
	}

	return &result.Data, nil
}

// CreateMediaItemStyledImage creates a relationship between a media item and a styled image
func (c *Client) CreateMediaItemStyledImage(mediaItemID, styledImageID string) error {
	endpoint := fmt.Sprintf("%s/relations/mediaitems_images", c.BaseURL)

	payload := map[string]interface{}{
		"data": map[string]interface{}{
			"mediaitems_id":   mediaItemID,
			"styledimages_id": styledImageID,
		},
	}

	resp, err := c.client.R().
		SetBody(payload).
		Post(endpoint)

	if err != nil {
		return fmt.Errorf("failed to create media item styled image relationship: %w", err)
	}

	if resp.StatusCode() != 200 {
		return fmt.Errorf("Directus API error: %s - %s", resp.Status(), string(resp.Body()))
	}

	return nil
}

// CreateMediaItem creates a new media item in Directus
func (c *Client) CreateMediaItem(mediaItem MediaItemCreate) (*MediaItem, error) {
	endpoint := fmt.Sprintf("%s/items/mediaitems", c.BaseURL)
	result := &struct {
		Data MediaItem `json:"data"`
	}{}

	// Remove the Images field before sending to avoid the one-to-many update issue
	type mediaItemCreatePayload struct {
		Label           string `json:"label"`
		Type            string `json:"type"`
		AssetID         int64  `json:"asset_id"`
		Title           string `json:"title"`
		ParentEpisodeID *int   `json:"parent_episode_id,omitempty"`
		ParentStartsAt  *int64 `json:"parent_starts_at,omitempty"`
		ParentEndsAt    *int64 `json:"parent_ends_at,omitempty"`
	}

	payload := mediaItemCreatePayload{
		Label:           mediaItem.Label,
		Type:            mediaItem.Type,
		AssetID:         mediaItem.AssetID,
		Title:           mediaItem.Title,
		ParentEpisodeID: mediaItem.ParentEpisodeID,
		ParentStartsAt:  mediaItem.ParentStartsAt,
		ParentEndsAt:    mediaItem.ParentEndsAt,
	}

	resp, err := c.client.R().
		SetResult(result).
		SetBody(payload).
		Post(endpoint)

	if err != nil {
		return nil, fmt.Errorf("failed to create media item: %w", err)
	}

	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("Directus API error: %s - %s", resp.Status(), string(resp.Body()))
	}

	return &result.Data, nil
}

// UploadFile uploads a file to Directus and returns the file information
func (c *Client) UploadFile(filePath string) (*File, error) {
	fileBytes, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	filename := filepath.Base(filePath)
	result := &struct {
		Data File `json:"data"`
	}{}

	resp, err := c.client.R().
		SetResult(result).
		SetFileReader("file", filename, bytes.NewReader(fileBytes)).
		SetMultipartFormData(map[string]string{
			"type":   "image/jpeg",
			"folder": "4a8eb774-62ed-404d-9ab1-295797f6383f", /// TODO: get this from env config or from a call
		}).
		Post(fmt.Sprintf("%s/files", c.BaseURL))

	if err != nil {
		return nil, fmt.Errorf("failed to upload file: %w", err)
	}

	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("Directus API error: %s", resp.Status())
	}

	if result.Data.ID == "" {
		return nil, fmt.Errorf("invalid response from Directus: missing file ID")
	}

	return &result.Data, nil
}
