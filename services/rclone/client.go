package rclone

import "github.com/go-resty/resty/v2"

type Client struct {
	username string
	password string
	baseURL  string

	restyClient *resty.Client
}

func NewClient(baseURL, username, password string) *Client {
	client := resty.New()
	client.SetBasicAuth(username, password)
	client.SetBaseURL(baseURL)
	client.SetDisableWarn(true)
	client.SetRetryCount(10)

	return &Client{
		baseURL:     baseURL,
		username:    username,
		password:    password,
		restyClient: client,
	}
}
