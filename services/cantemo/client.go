package cantemo

import (
	"github.com/go-resty/resty/v2"
	"strings"
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
			Get("/vs/item/download/VX-486350/?shape=VX-978080")

		if err != nil {
			return nil, err
		}

		return res.Result().(*Transcription), nil
	}

	return &Transcription{}, nil
}
