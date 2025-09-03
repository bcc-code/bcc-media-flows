package vizualizer

import (
	"fmt"
	"github.com/go-resty/resty/v2"
)

// Client is a lightweight REST client for the Music Vizualizer service.
// It mirrors the minimal pattern used in services/notion.
//
// BaseURL example: "http://vizualizer.lan.bcc.media"
// If the service requires auth in the future, extend this with headers.

type Client struct {
	BaseURL string
	client  *resty.Client
}

// NewClient constructs a vizualizer API client.
func NewClient(baseURL string) (*Client, error) {
	if baseURL == "" {
		return nil, fmt.Errorf("vizualizer baseURL not set")
	}
	c := resty.New()
	return &Client{BaseURL: baseURL, client: c}, nil
}

// CreateVisualizationRequest models the POST body for creating a visualization.
// See README: POST /api/visualize
type CreateVisualizationRequest struct {
    AudioPath    string `json:"audio_path"`
    OutputPath   string `json:"output_path"`
    Width        int    `json:"width,omitempty"`
    Height       int    `json:"height,omitempty"`
    FPS          int    `json:"fps,omitempty"`
    IncludeAudio bool   `json:"include_audio,omitempty"`
}

// CreateVisualizationResponse is returned by POST /api/visualize
type CreateVisualizationResponse struct {
    JobID      string `json:"job_id"`
    Status     string `json:"status"`
    Message    string `json:"message"`
    OutputPath string `json:"output_path"`
}

// JobStatusResponse models a job returned by GET /api/status/{job_id} and /api/jobs
type JobStatusResponse struct {
    JobID      string `json:"job_id"`
    Status     string `json:"status"`
    Progress   int    `json:"progress"`
    Message    string `json:"message"`
    OutputFile string `json:"output_file"`
    CreatedAt  int64  `json:"created_at"`
}

// CreateVisualization starts a new visualization job from a local audio file.
func (c *Client) CreateVisualization(req CreateVisualizationRequest) (*CreateVisualizationResponse, error) {
    url := c.BaseURL + "/api/visualize"
    var out CreateVisualizationResponse
    resp, err := c.client.R().SetBody(req).SetResult(&out).Post(url)
    if err != nil {
        return nil, err
    }
    if resp.StatusCode() < 200 || resp.StatusCode() >= 300 {
        return nil, fmt.Errorf("vizualizer create failed: %s, body: %s", resp.Status(), resp.String())
    }
    return &out, nil
}

// GetJob fetches the status of a specific visualization job.
func (c *Client) GetJob(jobID string) (*JobStatusResponse, error) {
    if jobID == "" {
        return nil, fmt.Errorf("jobID is required")
    }
    url := c.BaseURL + "/api/status/" + jobID
    var out JobStatusResponse
    resp, err := c.client.R().SetResult(&out).Get(url)
    if err != nil {
        return nil, err
    }
    if resp.StatusCode() != 200 {
        return nil, fmt.Errorf("vizualizer get job failed: %s, body: %s", resp.Status(), resp.String())
    }
    return &out, nil
}

// ListJobs returns all visualization jobs.
func (c *Client) ListJobs() ([]JobStatusResponse, error) {
    url := c.BaseURL + "/api/jobs"
    var out []JobStatusResponse
    resp, err := c.client.R().SetResult(&out).Get(url)
    if err != nil {
        return nil, err
    }
    if resp.StatusCode() != 200 {
        return nil, fmt.Errorf("vizualizer list jobs failed: %s, body: %s", resp.Status(), resp.String())
    }
    return out, nil
}

// Health pings the health endpoint. Returns nil if healthy.
func (c *Client) Health() error {
    url := c.BaseURL + "/api/health"
    resp, err := c.client.R().Get(url)
    if err != nil {
        return err
    }
    if resp.StatusCode() != 200 {
        return fmt.Errorf("vizualizer health failed: %s, body: %s", resp.Status(), resp.String())
    }
    return nil
}
