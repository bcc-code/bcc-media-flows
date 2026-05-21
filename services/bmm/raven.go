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

// RavenPlaylist mirrors the curated-playlist documents in the BMM `Playlists` collection.
// Track IDs are not stored here — follow TrackCollectionID into the `TrackCollections` collection.
// TrackCollectionID is nil for auto/dynamic playlists (e.g. "Top Songs").
type RavenPlaylist struct {
	ID                int                        `json:"Id"`
	TrackCollectionID *int                       `json:"TrackCollectionId"`
	Tags              []string                   `json:"Tags"`
	Translations      []RavenPlaylistTranslation `json:"Translations"`
}

// RavenPlaylistTranslation uses lowercase keys (`language`, `title`) — that's how the
// `Playlists` documents encode translations, unlike `Tracks` which use TitleCase.
type RavenPlaylistTranslation struct {
	Language    string `json:"language"`
	Title       string `json:"title"`
	Description string `json:"Description"`
}

// PlaylistName returns the playlist's title in the requested language, falling back to
// English, then the first non-empty title, then "".
func (p *RavenPlaylist) PlaylistName(language string) string {
	if p == nil {
		return ""
	}
	for _, tr := range p.Translations {
		if tr.Language == language && tr.Title != "" {
			return tr.Title
		}
	}
	for _, tr := range p.Translations {
		if tr.Language == "en" && tr.Title != "" {
			return tr.Title
		}
	}
	for _, tr := range p.Translations {
		if tr.Title != "" {
			return tr.Title
		}
	}
	return ""
}

type RavenTrackCollection struct {
	ID              int                   `json:"Id"`
	Name            string                `json:"Name"`
	TrackReferences []RavenTrackReference `json:"TrackReferences"`
}

type RavenTrackReference struct {
	ID       int    `json:"Id"`
	Language string `json:"Language"`
}

func (c *RavenClient) LoadTrackByBmmID(ctx context.Context, bmmTrackID int) (*RavenTrack, error) {
	q := fmt.Sprintf("from Tracks where Id = %d", bmmTrackID)

	results, err := c.queryTracks(ctx, q)
	if err != nil {
		return nil, err
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("no track found with BMM track ID %d", bmmTrackID)
	}
	if len(results) > 1 {
		return nil, fmt.Errorf("found %d tracks with BMM track ID %d; expected exactly 1", len(results), bmmTrackID)
	}
	return &results[0], nil
}

func (c *RavenClient) LoadTracksByAlbumID(ctx context.Context, albumID int) ([]RavenTrack, error) {
	q := fmt.Sprintf("from Tracks where ParentId = %d", albumID)

	results, err := c.queryTracks(ctx, q)
	if err != nil {
		return nil, err
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("no tracks found for album ID %d", albumID)
	}
	return results, nil
}

func (c *RavenClient) LoadTracksByIDs(ctx context.Context, ids []int) ([]RavenTrack, error) {
	if len(ids) == 0 {
		return nil, fmt.Errorf("LoadTracksByIDs: ids must not be empty")
	}

	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = fmt.Sprintf("%d", id)
	}
	q := fmt.Sprintf("from Tracks where Id in (%s)", strings.Join(parts, ", "))

	return c.queryTracks(ctx, q)
}

func (c *RavenClient) LoadPlaylistByID(ctx context.Context, playlistID int) (*RavenPlaylist, error) {
	q := fmt.Sprintf("from Playlists where Id = %d", playlistID)

	results, err := queryRaven[RavenPlaylist](ctx, c, q)
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("no playlist found with ID %d", playlistID)
	}
	if len(results) > 1 {
		return nil, fmt.Errorf("found %d playlists with ID %d; expected exactly 1", len(results), playlistID)
	}
	return &results[0], nil
}

func (c *RavenClient) LoadTrackCollectionByID(ctx context.Context, id int) (*RavenTrackCollection, error) {
	q := fmt.Sprintf("from TrackCollections where Id = %d", id)

	results, err := queryRaven[RavenTrackCollection](ctx, c, q)
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("no track collection found with ID %d", id)
	}
	if len(results) > 1 {
		return nil, fmt.Errorf("found %d track collections with ID %d; expected exactly 1", len(results), id)
	}
	return &results[0], nil
}

// LoadTrackIDsForPlaylist returns the playlist document and the deduped BMM track IDs that
// belong to it, preserving playlist order. Errors if the playlist has no associated track
// collection (some playlists, like "Top Songs", are dynamically generated and can't be
// enumerated this way).
func (c *RavenClient) LoadTrackIDsForPlaylist(ctx context.Context, playlistID int) (*RavenPlaylist, []int, error) {
	pl, err := c.LoadPlaylistByID(ctx, playlistID)
	if err != nil {
		return nil, nil, err
	}
	if pl.TrackCollectionID == nil {
		return pl, nil, fmt.Errorf("playlist %d has no track collection (likely a dynamic/auto-generated playlist)", playlistID)
	}

	tc, err := c.LoadTrackCollectionByID(ctx, *pl.TrackCollectionID)
	if err != nil {
		return pl, nil, fmt.Errorf("track collection %d for playlist %d: %w", *pl.TrackCollectionID, playlistID, err)
	}

	seen := make(map[int]struct{}, len(tc.TrackReferences))
	ids := make([]int, 0, len(tc.TrackReferences))
	for _, ref := range tc.TrackReferences {
		if _, ok := seen[ref.ID]; ok {
			continue
		}
		seen[ref.ID] = struct{}{}
		ids = append(ids, ref.ID)
	}
	if len(ids) == 0 {
		return pl, nil, fmt.Errorf("playlist %d (track collection %d) has no tracks", playlistID, *pl.TrackCollectionID)
	}
	return pl, ids, nil
}

func (c *RavenClient) queryTracks(ctx context.Context, query string) ([]RavenTrack, error) {
	return queryRaven[RavenTrack](ctx, c, query)
}

func queryRaven[T any](ctx context.Context, c *RavenClient, query string) ([]T, error) {
	endpoint := fmt.Sprintf("%s/databases/%s/queries", c.cfg.URL, c.cfg.Database)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint+"?"+url.Values{"query": {query}}.Encode(), nil)
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

	var out ravenQueryResponse[T]
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("failed to decode ravendb response: %w", err)
	}

	return out.Results, nil
}
