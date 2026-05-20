package bmm

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

type RavenConfig struct {
	URL         string
	Database    string
	CertPath    string
	CertKeyPath string
}

func LoadRavenConfigFromEnv() (RavenConfig, error) {
	cfg := RavenConfig{
		URL:         strings.TrimRight(os.Getenv("RAVENDB_URL"), "/"),
		Database:    os.Getenv("RAVENDB_DATABASE"),
		CertPath:    os.Getenv("RAVENDB_CERT_PATH"),
		CertKeyPath: os.Getenv("RAVENDB_CERT_KEY_PATH"),
	}
	if cfg.URL == "" {
		return cfg, fmt.Errorf("RAVENDB_URL is required")
	}
	if cfg.Database == "" {
		return cfg, fmt.Errorf("RAVENDB_DATABASE is required")
	}
	return cfg, nil
}

type RavenClient struct {
	cfg  RavenConfig
	http *http.Client
}

func NewRavenClient(cfg RavenConfig) (*RavenClient, error) {
	transport := &http.Transport{}

	if cfg.CertPath != "" && cfg.CertKeyPath != "" {
		cert, err := tls.LoadX509KeyPair(cfg.CertPath, cfg.CertKeyPath)
		if err != nil {
			return nil, fmt.Errorf("failed to load client certificate: %w", err)
		}
		transport.TLSClientConfig = &tls.Config{
			Certificates: []tls.Certificate{cert},
		}
	}

	return &RavenClient{
		cfg: cfg,
		http: &http.Client{
			Transport: transport,
			Timeout:   30 * time.Second,
		},
	}, nil
}

type ravenQueryResponse[T any] struct {
	TotalResults int `json:"TotalResults"`
	Results      []T `json:"Results"`
}

// RavenTrack mirrors the relevant fields of a BMM Tracks document.
// Unknown fields are intentionally dropped — the mapping layer only consumes what's listed here.
type RavenTrack struct {
	ID               int                `json:"Id"`
	MediabankenID    string             `json:"MediabankenId"`
	OriginalLanguage string             `json:"OriginalLanguage"`
	ParentID         int                `json:"ParentId"`
	PublishedAt      string             `json:"PublishedAt"`
	RecordedAt       string             `json:"RecordedAt"`
	Subtype          string             `json:"Subtype"`
	Tags             []string           `json:"Tags"`
	Rel              []RavenRel         `json:"Rel"`
	Translations     []RavenTranslation `json:"Translations"`
}

type RavenRel struct {
	Type      string `json:"Type"`
	Name      string `json:"Name"`
	ID        int    `json:"Id"`
	Timestamp int    `json:"Timestamp,omitempty"`
	URL       string `json:"Url,omitempty"`
}

type RavenTranslation struct {
	Language  string             `json:"Language"`
	IsVisible bool               `json:"IsVisible"`
	Title     string             `json:"Title"`
	Media     []RavenMediaGroup  `json:"Media"`
	Meta      RavenTranslationMeta `json:"_meta"`
}

type RavenMediaGroup struct {
	Type      string         `json:"Type"`
	IsVisible bool           `json:"IsVisible"`
	Files     []RavenMediaFile `json:"Files"`
}

type RavenMediaFile struct {
	MimeType string `json:"MimeType"`
	Size     int64  `json:"Size"`
	Duration int    `json:"Duration"`
	Path     string `json:"Path"`
	Bitrate  int    `json:"Bitrate"`
}

type RavenTranslationMeta struct {
	Title     string `json:"Title"`
	Language  string `json:"Language"`
	Album     string `json:"Album"`
	Copyright string `json:"Copyright"`
}

func (c *RavenClient) LoadTrackByBmmID(ctx context.Context, bmmTrackID int) (*RavenTrack, error) {
	q := fmt.Sprintf("from Tracks where Id = %d", bmmTrackID)

	endpoint := fmt.Sprintf("%s/databases/%s/queries", c.cfg.URL, c.cfg.Database)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint+"?"+url.Values{"query": {q}}.Encode(), nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ravendb request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("ravendb returned %s: %s", resp.Status, string(body))
	}

	var out ravenQueryResponse[RavenTrack]
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("failed to decode ravendb response: %w", err)
	}

	if len(out.Results) == 0 {
		return nil, fmt.Errorf("no track found with BMM track ID %d", bmmTrackID)
	}
	if len(out.Results) > 1 {
		return nil, fmt.Errorf("found %d tracks with BMM track ID %d; expected exactly 1", len(out.Results), bmmTrackID)
	}
	return &out.Results[0], nil
}
