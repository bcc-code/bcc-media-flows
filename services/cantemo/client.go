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
