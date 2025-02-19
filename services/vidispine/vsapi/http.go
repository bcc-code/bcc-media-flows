package vsapi

import (
	"github.com/go-resty/resty/v2"
	"time"
)

type Client struct {
	baseURL     string
	username    string
	password    string
	restyClient *resty.Client
}

func NewClient(baseURL string, username string, password string) *Client {
	client := resty.New()
	client.SetBasicAuth(username, password)
	client.SetBaseURL(baseURL)
	client.SetHeader("accept", "application/json")
	client.SetDisableWarn(true)
	client.SetTimeout(10 * time.Second)
	client.SetRetryCount(5)

	return &Client{
		baseURL:     baseURL,
		username:    username,
		password:    password,
		restyClient: client,
	}
}

type IDOnlyResult struct {
	VXID string `json:"id"`
}
