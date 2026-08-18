package vizualizer

import (
	"fmt"

	"github.com/go-resty/resty/v2"

	"github.com/bcc-code/bcc-media-flows/internal/httpx"
)

const serviceName = "vizualizer"

// Client is a lightweight REST client for the Music Vizualizer service.
//
// BaseURL example: "http://vizualizer.lan.bcc.media"
// If the service requires auth in the future, extend this with headers.

type Client struct {
	BaseURL string
	client  *resty.Client
}

// NewClient constructs a vizualizer API client.
type Config interface {
	Vizualizer() string
}

func NewFromConfig(cfg Config) (*Client, error) {
	return NewClient(cfg.Vizualizer())
}

func NewClient(baseURL string) (*Client, error) {
	if baseURL == "" {
		return nil, fmt.Errorf("vizualizer baseURL not set")
	}

	client := httpx.New(httpx.Config{
		Service: serviceName,
		BaseURL: baseURL,
	})

	return &Client{BaseURL: baseURL, client: client}, nil
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
	JobID      string  `json:"job_id"`
	Status     string  `json:"status"`
	Progress   int     `json:"progress"`
	Message    string  `json:"message"`
	OutputFile string  `json:"output_file"`
	CreatedAt  float64 `json:"created_at"`
}

// CreateVisualization starts a new visualization job from a local audio file.
func (c *Client) CreateVisualization(req CreateVisualizationRequest) (*CreateVisualizationResponse, error) {
	var out CreateVisualizationResponse
	_, err := c.client.R().SetBody(req).SetResult(&out).Post("/api/visualize")
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// GetJob fetches the status of a specific visualization job.
func (c *Client) GetJob(jobID string) (*JobStatusResponse, error) {
	if jobID == "" {
		return nil, fmt.Errorf("jobID is required")
	}
	var out JobStatusResponse
	_, err := c.client.R().SetResult(&out).Get("/api/status/" + jobID)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// ListJobs returns all visualization jobs.
func (c *Client) ListJobs() ([]JobStatusResponse, error) {
	var out []JobStatusResponse
	_, err := c.client.R().SetResult(&out).Get("/api/jobs")
	if err != nil {
		return nil, err
	}
	return out, nil
}

// Health pings the health endpoint. Returns nil if healthy.
func (c *Client) Health() error {
	_, err := c.client.R().Get("/api/health")
	return err
}
