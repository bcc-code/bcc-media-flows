package activities

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/temporal"

	"github.com/bcc-code/bcc-media-flows/paths"
	"github.com/bcc-code/bcc-media-flows/services/mediabrain"
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
	// Model that produced the transcript. Empty leaves the revision without one.
	Model string `json:",omitempty"`
}

type ImportTranscriptToMediaBrainResult struct {
	AssetID   string
	VersionID string
	TrackID   string
	Revision  int
}

// ImportTranscriptToMediaBrain adds the SRT as a new TRANSCRIPT revision on the latest
// version of the Brain asset imported from the Vidispine item.
func (a *MediaBrainActivities) ImportTranscriptToMediaBrain(ctx context.Context, params ImportTranscriptToMediaBrainParams) (*ImportTranscriptToMediaBrainResult, error) {
	logger := activity.GetLogger(ctx)
	activity.RecordHeartbeat(ctx, "ImportTranscriptToMediaBrain")

	if a.Client == nil {
		return nil, temporal.NewNonRetryableApplicationError("media brain client not configured", "mediabrain_not_configured", nil)
	}

	content, err := os.ReadFile(params.SRTPath.Local())
	if err != nil {
		return nil, fmt.Errorf("reading transcript: %w", err)
	}

	asset, err := a.Client.FindAssetByLegacyID(ctx, params.VXID)
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
		Format:    mediabrain.SubtitleFormatSRT,
		Origin:    mediabrain.TrackOriginASR,
		Content:   string(content),
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

// nonRetryableMediaBrainError stops retries for answers that a retry gets again. Not
// UNAUTHENTICATED: that is also what an expired token gets, and the next attempt
// fetches a new one.
func nonRetryableMediaBrainError(err error) error {
	for _, permanent := range []error{
		mediabrain.ErrAssetNotFound,
		mediabrain.ErrAmbiguousAsset,
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
