package activities

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/temporal"

	"github.com/bcc-code/bcc-media-flows/paths"
	"github.com/bcc-code/bcc-media-flows/services/mediabrain"
	"github.com/bcc-code/bcc-media-flows/services/srt"
)

type MediaBrainActivities struct {
	Client *mediabrain.Client
}

// MediaBrain gets its client at boot, from the configuration.
var MediaBrain = &MediaBrainActivities{}

type ImportTranscriptToMediaBrainParams struct {
	// VXID finds the Brain asset: Brain keeps it among the asset's legacy ids.
	VXID string
	// Language of the transcript. Empty or "auto" leaves the track without one.
	Language string
	SRTPath  paths.Path
	// JSONPath is the transcriber's JSON. Its word timing is attached to the cues of the
	// SRT. Nil, unreadable or broken leaves the cues without words.
	JSONPath *paths.Path `json:",omitempty"`
	// Model that produced the transcript. Empty leaves the revision without one.
	Model string `json:",omitempty"`
}

type ImportTranscriptToMediaBrainResult struct {
	AssetID   string
	VersionID string
	TrackID   string
	Revision  int
}

// ImportTranscriptToMediaBrain adds the SRT, with the words of the transcriber's JSON,
// as a new TRANSCRIPT revision on the latest version of the Brain asset imported from
// the Vidispine item. An item Brain does not have yet is imported first, as raw
// material without a project.
func (a *MediaBrainActivities) ImportTranscriptToMediaBrain(ctx context.Context, params ImportTranscriptToMediaBrainParams) (*ImportTranscriptToMediaBrainResult, error) {
	logger := activity.GetLogger(ctx)
	activity.RecordHeartbeat(ctx, "ImportTranscriptToMediaBrain")

	if a.Client == nil {
		return nil, temporal.NewNonRetryableApplicationError("media brain client not configured", "mediabrain_not_configured", nil)
	}

	raw, err := os.ReadFile(params.SRTPath.Local())
	if err != nil {
		return nil, fmt.Errorf("reading transcript: %w", err)
	}
	cues, err := srt.Parse(raw)
	if err != nil {
		return nil, temporal.NewNonRetryableApplicationError(err.Error(), "invalid_transcript", err)
	}
	// Brain refuses a whole file for one cue without length, and Whisper writes those.
	cues, changed := srt.Repair(cues)
	if changed > 0 {
		logger.Warn("Repaired transcript cues for Media Brain", "vxid", params.VXID, "cues", changed)
	}

	transcript, fixes := mediabrain.NewTranscript(cues, readTranscriptWords(ctx, params))
	if fixes.Total() > 0 {
		logger.Warn("Repaired transcript words for Media Brain", "vxid", params.VXID,
			"startsClamped", fixes.StartsClamped, "endsRaised", fixes.EndsRaised,
			"confidencesDropped", fixes.ConfidencesDropped, "wordsDropped", fixes.WordsDropped)
	}
	content, err := transcriptContent(ctx, params.VXID, transcript)
	if err != nil {
		return nil, temporal.NewNonRetryableApplicationError(err.Error(), "invalid_transcript", err)
	}

	asset, err := a.findOrImportAsset(ctx, params.VXID)
	if err != nil {
		return nil, nonRetryableMediaBrainError(err)
	}
	version, err := asset.LatestVersion()
	if err != nil {
		return nil, nonRetryableMediaBrainError(err)
	}

	input := mediabrain.ImportSubtitlesInput{
		VersionID: version.ID,
		Kind:      mediabrain.TrackKindTranscript,
		Format:    mediabrain.SubtitleFormatJSON,
		Origin:    mediabrain.TrackOriginASR,
		Content:   content,
	}
	if model := strings.TrimSpace(params.Model); model != "" {
		input.Model = &model
	}
	if lang := strings.TrimSpace(params.Language); lang != "" && lang != "auto" {
		input.Language = &lang
	}

	track, err := a.Client.ImportSubtitles(ctx, input)
	if err != nil {
		return nil, nonRetryableMediaBrainError(err)
	}

	logger.Info("Imported transcript to Media Brain", "vxid", params.VXID, "asset", asset.ID, "version", version.ID, "track", track.ID, "revision", track.Revision, "model", params.Model)

	return &ImportTranscriptToMediaBrainResult{
		AssetID:   asset.ID,
		VersionID: version.ID,
		TrackID:   track.ID,
		Revision:  track.Revision,
	}, nil
}

// readTranscriptWords returns the words of the transcriber's JSON. Words are secondary
// to the cues, so a file that cannot be read gives none instead of failing the import.
func readTranscriptWords(ctx context.Context, params ImportTranscriptToMediaBrainParams) []mediabrain.TranscriptWord {
	if params.JSONPath == nil {
		return nil
	}
	logger := activity.GetLogger(ctx)
	data, err := os.ReadFile(params.JSONPath.Local())
	if err != nil {
		logger.Warn("Importing transcript without words", "vxid", params.VXID, "error", err)
		return nil
	}
	words, err := mediabrain.ReadWords(data)
	if err != nil {
		logger.Warn("Importing transcript without words", "vxid", params.VXID, "error", err)
		return nil
	}
	return words
}

// transcriptContent is the transcript as Brain imports it. One that is too large with
// its words is sent without them, since the cues matter more.
func transcriptContent(ctx context.Context, vxID string, transcript mediabrain.Transcript) (string, error) {
	content, err := json.Marshal(transcript)
	if err != nil {
		return "", fmt.Errorf("encoding transcript: %w", err)
	}
	if len(content) <= mediabrain.MaxContentBytes {
		return string(content), nil
	}

	withWords := len(content)
	content, err = json.Marshal(transcript.WithoutWords())
	if err != nil {
		return "", fmt.Errorf("encoding transcript: %w", err)
	}
	if len(content) < withWords {
		activity.GetLogger(ctx).Warn("Transcript too large for Media Brain with words, importing it without", "vxid", vxID, "bytes", withWords, "limit", mediabrain.MaxContentBytes)
	}
	return string(content), nil
}

// findOrImportAsset imports the Vidispine item when Brain has no asset for it yet.
func (a *MediaBrainActivities) findOrImportAsset(ctx context.Context, vxID string) (*mediabrain.Asset, error) {
	asset, err := a.Client.FindAssetByLegacyID(ctx, vxID)
	if !errors.Is(err, mediabrain.ErrAssetNotFound) {
		return asset, err
	}

	typeID, err := a.Client.AssetTypeID(ctx, mediabrain.AssetTypeRawMaterial)
	if err != nil {
		return nil, err
	}
	asset, err = a.Client.ImportFromVidispine(ctx, vxID, typeID)
	if errors.Is(err, mediabrain.ErrConflict) {
		// Imported in the meantime, or by an earlier attempt whose answer was lost.
		return a.Client.FindAssetByLegacyID(ctx, vxID)
	}
	if err != nil {
		return nil, err
	}
	activity.GetLogger(ctx).Info("Imported Vidispine item into Media Brain", "vxid", vxID, "asset", asset.ID)
	return asset, nil
}

// nonRetryableMediaBrainError stops retries for answers that a retry gets again. Not
// UNAUTHENTICATED: that is also what an expired token gets, and the next attempt
// fetches a new one.
func nonRetryableMediaBrainError(err error) error {
	for _, permanent := range []error{
		mediabrain.ErrAssetNotFound,
		mediabrain.ErrAmbiguousAsset,
		mediabrain.ErrAssetTypeNotFound,
		mediabrain.ErrNoVersion,
		mediabrain.ErrForbidden,
		mediabrain.ErrNotFound,
		mediabrain.ErrInvalidInput,
	} {
		if errors.Is(err, permanent) {
			return temporal.NewNonRetryableApplicationError(err.Error(), "mediabrain_refused", err)
		}
	}
	return err
}
