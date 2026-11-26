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

// TrackOrigin is how a revision's text came to be, as Brain's TrackOrigin enum names it.
// An import may only send IMPORT or ASR; MANUAL is what Brain records for edits.
type TrackOrigin enum.Member[string]

var (
	TrackOriginImport = TrackOrigin{Value: "IMPORT"}
	TrackOriginASR    = TrackOrigin{Value: "ASR"}
	TrackOriginManual = TrackOrigin{Value: "MANUAL"}
	TrackOrigins      = enum.New(TrackOriginImport, TrackOriginASR, TrackOriginManual)
)

func (o TrackOrigin) String() string { return o.Value }

//goland:noinspection GoMixedReceiverTypes
func (o TrackOrigin) MarshalJSON() ([]byte, error) { return enumjson.Marshal(o) }

//goland:noinspection GoMixedReceiverTypes
func (o *TrackOrigin) UnmarshalJSON(data []byte) error { return enumjson.UnmarshalOpen(data, o) }

// SubtitleFormat is the text format of an imported track, as Brain's SubtitleFormat
// enum names it.
type SubtitleFormat enum.Member[string]

var (
	SubtitleFormatSRT = SubtitleFormat{Value: "SRT"}
	SubtitleFormatVTT = SubtitleFormat{Value: "VTT"}
	// SubtitleFormatJSON is a Transcript: cues that carry their word timing.
	SubtitleFormatJSON = SubtitleFormat{Value: "JSON"}
	SubtitleFormats    = enum.New(SubtitleFormatSRT, SubtitleFormatVTT, SubtitleFormatJSON)
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

// AssetTypeRawMaterial is the name of Brain's asset type for source recordings.
const AssetTypeRawMaterial = "Raw material"

var ErrAssetTypeNotFound = errors.New("media brain: no asset type has this name")

type AssetType struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

const assetTypesQuery = `query AssetTypes { assetTypes { id name } }`

// AssetTypeID returns the id of the asset type with this name.
func (c *Client) AssetTypeID(ctx context.Context, name string) (int, error) {
	var out struct {
		AssetTypes []AssetType `json:"assetTypes"`
	}
	if err := c.do(ctx, assetTypesQuery, nil, &out); err != nil {
		return 0, fmt.Errorf("listing asset types: %w", err)
	}
	for _, t := range out.AssetTypes {
		if t.Name == name {
			return t.ID, nil
		}
	}
	return 0, fmt.Errorf("%w: %s", ErrAssetTypeNotFound, name)
}

const importFromVidispineMutation = `mutation ImportFromVidispine($input: VidispineImportInput!) {
  importFromVidispine(input: $input) {
    id
    title
    versions(first: 500) { nodes { id number } }
  }
}`

// ImportFromVidispine creates the asset for a Vidispine item, without a project. Brain
// allows it to administrators only, and answers ErrConflict when the item is already
// imported.
func (c *Client) ImportFromVidispine(ctx context.Context, itemID string, typeID int) (*Asset, error) {
	var out struct {
		ImportFromVidispine Asset `json:"importFromVidispine"`
	}
	input := map[string]any{"itemId": itemID, "typeId": typeID}
	if err := c.do(ctx, importFromVidispineMutation, map[string]any{"input": input}, &out); err != nil {
		return nil, fmt.Errorf("importing %s from Vidispine: %w", itemID, err)
	}
	return &out.ImportFromVidispine, nil
}

type ImportSubtitlesInput struct {
	VersionID string `json:"versionId"`
	// BCP 47 tag. Nil leaves the track without a language.
	Language *string        `json:"language,omitempty"`
	Kind     TrackKind      `json:"kind"`
	Format   SubtitleFormat `json:"format"`
	// Zero lets Brain record IMPORT.
	Origin TrackOrigin `json:"origin,omitzero"`
	// The model that produced the text. Nil leaves the revision without one.
	Model *string `json:"model,omitempty"`
	// The subtitle text. Brain refuses more than MaxContentBytes with INVALID_INPUT.
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
