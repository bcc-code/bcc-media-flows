package baton

import "github.com/go-resty/resty/v2"

type Client struct {
	baseURL     string
	restyClient *resty.Client
}

func NewClient(baseURL string) *Client {
	client := resty.New()
	client.SetBaseURL(baseURL)
	client.SetHeader("accept", "application/json")
	client.SetDisableWarn(true)

	return &Client{
		baseURL:     baseURL,
		restyClient: client,
	}
}
