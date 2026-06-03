package clickup

import (
	"encoding/json"
	"fmt"

	"github.com/go-resty/resty/v2"
)

// defaultBaseURL is ClickUp's internal "frontdoor" host that backs the public
// share UI. It is NOT the documented public API (api.clickup.com/api/v2) and
// carries no stability guarantees, but it serves public-view data authenticated
// only by a share token — no API key required. The region segment (`-3`) can
// vary, so the base URL is configurable.
const defaultBaseURL = "https://frontdoor-prod-eu-west-1-3.clickup.com"

// Client reads tasks from a single public ClickUp view using its share token.
// It is read-only: a public share grants no write access.
type Client struct {
	baseURL     string
	workspaceID string
	viewID      string
	token       string
	client      *resty.Client
}

// NewClient builds a read-only client for one public view. workspaceID, viewID
// and token are all required (token is the public-share token from the view's
// share URL).
func NewClient(baseURL, workspaceID, viewID, token string) (*Client, error) {
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	if workspaceID == "" || viewID == "" || token == "" {
		return nil, fmt.Errorf("clickup public view requires workspaceID, viewID and token")
	}

	client := resty.New()
	client.SetHeader("Content-Type", "application/json")
	return &Client{
		baseURL:     baseURL,
		workspaceID: workspaceID,
		viewID:      viewID,
		token:       token,
		client:      client,
	}, nil
}

// Task is the downstream-facing shape, assembled from the two public-view calls.
type Task struct {
	ID           string
	Name         string
	CustomFields []CustomField
}

// CustomField carries a single field's value plus, for drop_down fields, the
// view's option definitions so DropDownName can resolve the selected option.
type CustomField struct {
	ID         string
	Name       string
	Type       string
	TypeConfig TypeConfig
	Value      json.RawMessage
}

type TypeConfig struct {
	Options []DropDownOption
}

type DropDownOption struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// fieldDefinition is the per-field metadata returned by the view-load call.
type fieldDefinition struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Type       string `json:"type"`
	TypeConfig struct {
		Options []DropDownOption `json:"options"`
	} `json:"type_config"`
}

// viewResponse is the GET …/public/view/{viewID} payload (the parts we use).
type viewResponse struct {
	CustomFields []fieldDefinition `json:"custom_fields"`
	LastPage     bool              `json:"last_page"`
	List         struct {
		Divisions []struct {
			Groups []struct {
				TaskIDs []string `json:"task_ids"`
			} `json:"groups"`
		} `json:"divisions"`
	} `json:"list"`
}

// publicTask is one task in the POST …/tasks response.
type publicTask struct {
	ID                string `json:"id"`
	Name              string `json:"name"`
	CustomFieldValues []struct {
		FieldID string          `json:"field_id"`
		Value   json.RawMessage `json:"value"`
	} `json:"customFieldValues"`
}

// tasksResponse is the POST …/public/view/{viewID}/tasks payload.
type tasksResponse struct {
	Tasks []publicTask `json:"tasks"`
}

// ListTasks returns every task in the configured public view, with the custom
// field values requested. It performs two steps: a view-load to get the field
// definitions and the task-ID list, then chunked task-value fetches.
func (c *Client) ListTasks() ([]Task, error) {
	defs, taskIDs, err := c.loadView()
	if err != nil {
		return nil, err
	}

	fieldDefs := make(map[string]fieldDefinition, len(defs))
	fieldIDs := make([]string, 0, len(defs))
	for _, d := range defs {
		fieldDefs[d.ID] = d
		fieldIDs = append(fieldIDs, d.ID)
	}

	const chunkSize = 100
	var all []Task
	for start := 0; start < len(taskIDs); start += chunkSize {
		end := min(start+chunkSize, len(taskIDs))

		pts, err := c.fetchTaskValues(taskIDs[start:end], fieldIDs)
		if err != nil {
			return nil, err
		}

		for _, pt := range pts {
			t := Task{ID: pt.ID, Name: pt.Name}
			for _, cfv := range pt.CustomFieldValues {
				def := fieldDefs[cfv.FieldID]
				t.CustomFields = append(t.CustomFields, CustomField{
					ID:         cfv.FieldID,
					Name:       def.Name,
					Type:       def.Type,
					TypeConfig: TypeConfig{Options: def.TypeConfig.Options},
					Value:      cfv.Value,
				})
			}
			all = append(all, t)
		}
	}

	return all, nil
}

// loadView fetches the view definition, returning the custom-field definitions
// and the full list of task IDs. It pages until last_page, guarding against a
// non-advancing page param by stopping once no new task IDs appear.
func (c *Client) loadView() ([]fieldDefinition, []string, error) {
	url := fmt.Sprintf("%s/view/v1/%s/public/view/%s", c.baseURL, c.workspaceID, c.viewID)

	var defs []fieldDefinition
	var taskIDs []string
	seen := map[string]bool{}

	for page := 0; ; page++ {
		var result viewResponse
		resp, err := c.client.R().
			SetQueryParams(map[string]string{
				"token": c.token,
				"page":  fmt.Sprintf("%d", page),
			}).
			SetResult(&result).
			Get(url)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to load ClickUp view: %w", err)
		}
		if resp.StatusCode() != 200 {
			return nil, nil, fmt.Errorf("ClickUp view load failed: %s, body: %s", resp.Status(), resp.String())
		}

		if page == 0 {
			defs = result.CustomFields
		}

		added := false
		for _, div := range result.List.Divisions {
			for _, grp := range div.Groups {
				for _, id := range grp.TaskIDs {
					if !seen[id] {
						seen[id] = true
						taskIDs = append(taskIDs, id)
						added = true
					}
				}
			}
		}

		if result.LastPage || !added {
			break
		}
	}

	return defs, taskIDs, nil
}

// fetchTaskValues fetches the requested custom field values for a batch of task IDs.
func (c *Client) fetchTaskValues(taskIDs, fieldIDs []string) ([]publicTask, error) {
	url := fmt.Sprintf("%s/view/v1/%s/public/view/%s/tasks", c.baseURL, c.workspaceID, c.viewID)

	payload := map[string]any{
		"context":           map[string]any{"listId": c.viewID, "dataType": "NextPage"},
		"task_ids":          taskIDs,
		"customFieldValues": fieldIDs,
		"token":             c.token,
	}

	var result tasksResponse
	resp, err := c.client.R().
		SetBody(payload).
		SetResult(&result).
		Post(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch ClickUp task values: %w", err)
	}
	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("ClickUp task fetch failed: %s, body: %s", resp.Status(), resp.String())
	}

	return result.Tasks, nil
}
