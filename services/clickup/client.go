package clickup

import (
	"encoding/json"
	"fmt"

	"github.com/go-resty/resty/v2"
)

const baseURL = "https://api.clickup.com/api/v2"

type Client struct {
	APIKey string
	client *resty.Client
}

func NewClient(apiKey string) (*Client, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("CLICKUP_API_KEY env var not set")
	}

	client := resty.New()
	client.SetHeader("Authorization", apiKey)
	client.SetHeader("Content-Type", "application/json")
	return &Client{
		APIKey: apiKey,
		client: client,
	}, nil
}

type Task struct {
	ID           string        `json:"id"`
	Name         string        `json:"name"`
	CustomFields []CustomField `json:"custom_fields"`
}

type CustomField struct {
	ID         string          `json:"id"`
	Name       string          `json:"name"`
	Type       string          `json:"type"`
	TypeConfig TypeConfig      `json:"type_config"`
	Value      json.RawMessage `json:"value"`
}

type TypeConfig struct {
	Options []DropDownOption `json:"options"`
}

type DropDownOption struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	OrderIndex int    `json:"orderindex"`
}

type CustomFieldFilter struct {
	FieldID  string `json:"field_id"`
	Operator string `json:"operator"`
	Value    string `json:"value"`
}

// ListTasks fetches tasks from a list, paginating until all tasks are returned.
// customFieldFilters are AND-combined and serialized as the `custom_fields` query parameter.
func (c *Client) ListTasks(listID string, customFieldFilters []CustomFieldFilter, includeClosed bool) ([]Task, error) {
	const pageSize = 100
	url := fmt.Sprintf("%s/list/%s/task", baseURL, listID)

	var filterParam string
	if len(customFieldFilters) > 0 {
		filterJSON, err := json.Marshal(customFieldFilters)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal custom field filter: %w", err)
		}
		filterParam = string(filterJSON)
	}

	var all []Task
	for page := 0; ; page++ {
		params := map[string]string{
			"subtasks": "true",
			"page":     fmt.Sprintf("%d", page),
		}
		if includeClosed {
			params["include_closed"] = "true"
		}
		if filterParam != "" {
			params["custom_fields"] = filterParam
		}

		var result struct {
			Tasks []Task `json:"tasks"`
		}
		resp, err := c.client.R().
			SetQueryParams(params).
			SetResult(&result).
			Get(url)
		if err != nil {
			return nil, fmt.Errorf("failed to list ClickUp tasks: %w", err)
		}
		if resp.StatusCode() != 200 {
			return nil, fmt.Errorf("ClickUp list tasks failed: %s, body: %s", resp.Status(), resp.String())
		}

		all = append(all, result.Tasks...)
		if len(result.Tasks) < pageSize {
			break
		}
	}

	return all, nil
}

// SetCustomFieldDropDown sets a drop_down custom field on a task to the given option UUID.
func (c *Client) SetCustomFieldDropDown(taskID, fieldID, optionID string) error {
	url := fmt.Sprintf("%s/task/%s/field/%s", baseURL, taskID, fieldID)
	payload := map[string]any{"value": optionID}

	resp, err := c.client.R().SetBody(payload).Post(url)
	if err != nil {
		return fmt.Errorf("failed to set ClickUp custom field: %w", err)
	}
	if resp.StatusCode() < 200 || resp.StatusCode() >= 300 {
		return fmt.Errorf("ClickUp set custom field failed: %s, body: %s", resp.Status(), resp.String())
	}
	return nil
}
