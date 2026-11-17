package mediabrain

import (
	"context"
	"errors"
	"fmt"

	"github.com/orsinium-labs/enum"

	"github.com/bcc-code/bcc-media-flows/internal/enumjson"
)

var (
	ErrAssetNotFound  = errors.New("media brain: no asset has this legacy id")
	ErrAmbiguousAsset = errors.New("media brain: more than one asset has this legacy id")
	ErrNoVersion      = errors.New("media brain: asset has no versions")
)

// TrackKind is what a subtitle track holds, as Brain's TrackKind enum names it.
type TrackKind enum.Member[string]

var (
	TrackKindSubtitle   = TrackKind{Value: "SUBTITLE"}
	TrackKindCC         = TrackKind{Value: "CC"}
	TrackKindTranscript = TrackKind{Value: "TRANSCRIPT"}
	TrackKinds          = enum.New(TrackKindSubtitle, TrackKindCC, TrackKindTranscript)
)

func (k TrackKind) String() string { return k.Value }

//goland:noinspection GoMixedReceiverTypes
func (k TrackKind) MarshalJSON() ([]byte, error) { return enumjson.Marshal(k) }

//goland:noinspection GoMixedReceiverTypes
func (k *TrackKind) UnmarshalJSON(data []byte) error { return enumjson.UnmarshalOpen(data, k) }

// SubtitleFormat is the text format of an imported track, as Brain's SubtitleFormat
// enum names it.
type SubtitleFormat enum.Member[string]

var (
	SubtitleFormatSRT = SubtitleFormat{Value: "SRT"}
	SubtitleFormatVTT = SubtitleFormat{Value: "VTT"}
	SubtitleFormats   = enum.New(SubtitleFormatSRT, SubtitleFormatVTT)
)

func (f SubtitleFormat) String() string { return f.Value }

//goland:noinspection GoMixedReceiverTypes
func (f SubtitleFormat) MarshalJSON() ([]byte, error) { return enumjson.Marshal(f) }

//goland:noinspection GoMixedReceiverTypes
func (f *SubtitleFormat) UnmarshalJSON(data []byte) error { return enumjson.UnmarshalOpen(data, f) }

type AssetVersion struct {
	ID     string `json:"id"`
	Number int    `json:"number"`
}

type Asset struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Versions struct {
		Nodes []AssetVersion `json:"nodes"`
	} `json:"versions"`
}

// LatestVersion is the version with the highest number.
func (a Asset) LatestVersion() (AssetVersion, error) {
	if len(a.Versions.Nodes) == 0 {
		return AssetVersion{}, fmt.Errorf("%w: asset %s", ErrNoVersion, a.ID)
	}
	latest := a.Versions.Nodes[0]
	for _, v := range a.Versions.Nodes[1:] {
		if v.Number > latest.Number {
			latest = v
		}
	}
	return latest, nil
}

const findAssetByLegacyIDQuery = `query FindAssetByLegacyID($legacyId: String!) {
  assets(filter: {legacyId: $legacyId}, first: 2) {
    nodes {
      id
      title
      versions(first: 500) { nodes { id number } }
    }
  }
}`

// FindAssetByLegacyID returns the one asset that carries legacyID, such as the VX id
// of an item imported from Vidispine.
func (c *Client) FindAssetByLegacyID(ctx context.Context, legacyID string) (*Asset, error) {
	var out struct {
		Assets struct {
			Nodes []Asset `json:"nodes"`
		} `json:"assets"`
	}
	err := c.do(ctx, findAssetByLegacyIDQuery, map[string]any{"legacyId": legacyID}, &out)
	if err != nil {
		return nil, fmt.Errorf("finding asset with legacy id %s: %w", legacyID, err)
	}
	switch len(out.Assets.Nodes) {
	case 0:
		return nil, fmt.Errorf("%w: %s", ErrAssetNotFound, legacyID)
	case 1:
		return &out.Assets.Nodes[0], nil
	default:
		return nil, fmt.Errorf("%w: %s", ErrAmbiguousAsset, legacyID)
	}
}

type ImportSubtitlesInput struct {
	VersionID string `json:"versionId"`
	// BCP 47 tag. Nil leaves the track without a language.
	Language *string        `json:"language,omitempty"`
	Kind     TrackKind      `json:"kind"`
	Format   SubtitleFormat `json:"format"`
	// The subtitle text. Brain refuses more than 5 MB with INVALID_INPUT.
	Content string `json:"content"`
}

type SubtitleTrack struct {
	ID          string `json:"id"`
	AssetID     string `json:"assetId"`
	RenditionID string `json:"renditionId"`
	Revision    int    `json:"revision"`
}

const importSubtitlesMutation = `mutation ImportSubtitles($input: ImportSubtitlesInput!) {
  importSubtitles(input: $input) { id assetId renditionId revision }
}`

// ImportSubtitles adds a new revision of the track for the input's language on the
// version.
func (c *Client) ImportSubtitles(ctx context.Context, input ImportSubtitlesInput) (*SubtitleTrack, error) {
	var out struct {
		ImportSubtitles SubtitleTrack `json:"importSubtitles"`
	}
	err := c.do(ctx, importSubtitlesMutation, map[string]any{"input": input}, &out)
	if err != nil {
		return nil, fmt.Errorf("importing subtitles to version %s: %w", input.VersionID, err)
	}
	return &out.ImportSubtitles, nil
}
