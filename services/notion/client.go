package notion

import (
	"encoding/json"
	"fmt"
	"github.com/go-resty/resty/v2"
)

type Client struct {
	APIKey string
	client *resty.Client
}

func NewClient(apiKey string) (*Client, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("NOTION_API_KEY env var not set")
	}

	client := resty.New()
	client.SetHeader("Authorization", "Bearer "+apiKey)
	client.SetHeader("Notion-Version", "2022-06-28")
	return &Client{
		APIKey: apiKey,
		client: client,
	}, nil
}

// FetchDatabaseMeta fetches the database info from Notion
func (c *Client) FetchDatabaseMeta(databaseID string) (map[string]interface{}, error) {
	url := fmt.Sprintf("https://api.notion.com/v1/databases/%s", databaseID)
	var db map[string]interface{}
	resp, err := c.client.R().SetResult(&db).Get(url)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("failed to fetch Notion DB: %s, body: %s", resp.Status(), resp.String())
	}
	return db, nil
}

// QueryDatabase fetches all rows from a Notion database
func (c *Client) QueryDatabase(databaseID string, filter string) ([]map[string]interface{}, error) {
	url := fmt.Sprintf("https://api.notion.com/v1/databases/%s/query", databaseID)
	var result struct {
		Results []map[string]interface{} `json:"results"`
	}

	body := struct {
		Filter json.RawMessage `json:"filter"`
	}{
		Filter: json.RawMessage(filter),
	}

	resp, err := c.client.R().SetBody(body).SetResult(&result).Post(url)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("failed to query Notion DB: %s, body: %s", resp.Status(), resp.String())
	}
	return result.Results, nil
}
