package cantemo

import (
	"strings"

	"github.com/go-resty/resty/v2"
)

type Client struct {
	baseURL     string
	restyClient *resty.Client
}

func NewClient(baseURL, authToken string) *Client {
	baseURL = strings.TrimSuffix(baseURL, "/")

	client := resty.New()
	client.SetBaseURL(baseURL)
	client.SetHeader("Auth-Token", authToken)
	client.SetHeader("Accept", "application/json")
	client.SetDisableWarn(true)

	return &Client{
		baseURL:     baseURL,
		restyClient: client,
	}
}

func (c *Client) AddRelation(parent, child string) error {
	req := c.restyClient.R()
	_, err := req.Post("/API/v2/items/" + parent + "/relation/" + child + "?type=portal_metadata_cascade&direction=D")

	return err
}

func (c *Client) GetFormats(itemID string) ([]Format, error) {

	req := c.restyClient.R().SetResult(&GetFormatsResponse{})
	res, err := req.Get("/API/v2/items/" + itemID + "/formats/")

	if err != nil {
		return nil, err
	}

	return res.Result().(*GetFormatsResponse).Formats, err
}

func (c *Client) GetMetadata(itemID string) (*ItemMetadata, error) {
	res, err := c.restyClient.R().SetResult(&ItemMetadata{}).
		Get("/API/v2/items/" + itemID + "/")

	if err != nil {
		return nil, err
	}

	return res.Result().(*ItemMetadata), nil
}

func (c *Client) GetPreviewUrl(itemID string) (string, error) {
	meta, err := c.GetMetadata(itemID)
	if err != nil {
		return "", err
	}

	for _, s := range meta.Previews.Shapes {
		return c.baseURL + s.URI, nil
	}

	return "", nil
}

func (c *Client) GetTranscriptionJSON(itemID string) (*Transcription, error) {
	formats, err := c.GetFormats(itemID)
	if err != nil {
		return nil, err
	}

	for _, format := range formats {
		if format.Name != "transcription_json" {
			continue
		}

		res, err := c.restyClient.R().
			SetResult(&Transcription{}).
			Get(format.DownloadURI)

		if err != nil {
			return nil, err
		}

		return res.Result().(*Transcription), nil
	}

	return &Transcription{}, nil
}

// GetFieldTags will return all tags for a given field
//
// The field probably needs to be a tags field (field_type: "tags")
func (c *Client) GetFieldTags(field string) ([]string, error) {
	type getTagsResponse struct {
		Tags []string `json:"tags"`
	}

	res, err := c.restyClient.R().
		SetResult(&getTagsResponse{}).
		Get("/API/v2/metadata-schema/fields/" + field + "/tags/?size=10000")

	if err != nil {
		return nil, err
	}

	result := res.Result().(*getTagsResponse)
	if result == nil {
		return nil, err
	}
	return result.Tags, nil
}
