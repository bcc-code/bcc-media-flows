package miscworkflows

import (
	"fmt"

	"github.com/bcc-code/bcc-media-flows/activities"
	vsactivity "github.com/bcc-code/bcc-media-flows/activities/vidispine"
	"github.com/bcc-code/bcc-media-flows/paths"
	"github.com/bcc-code/bcc-media-flows/services/vidispine/vscommon"
	wfutils "github.com/bcc-code/bcc-media-flows/utils/workflows"
	"go.temporal.io/sdk/workflow"
)

type ImportSidecarSubtitleInput struct {
	VXID     string     `json:"vxid"`
	FilePath paths.Path `json:"file_path"`
	Language string     `json:"language"`
}

// ImportSidecarSubtitle attaches a subtitle file to an asset as a sidecar.
//
// This is a workflow rather than a bare activity call so that callers can hand the
// import off and let it finish on its own: an activity is cancelled when the
// workflow that scheduled it completes, whereas a child workflow started through
// wfutils.WithAbandonChildOptions survives the parent closing. TranscribeVX uses
// it that way.
func ImportSidecarSubtitle(ctx workflow.Context, params ImportSidecarSubtitleInput) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting ImportSidecarSubtitle", "vxid", params.VXID, "language", params.Language)

	ctx = workflow.WithActivityOptions(ctx, wfutils.GetDefaultActivityOptions())

	// Vidispine resolves the subtitle group by name during sidecar import, and
	// stale instances from earlier imports make that fail with "ambiguous path
	// to group: stl_subtitle" — so remove them first.
	err := wfutils.Execute(ctx, activities.Vidispine.DeleteMetadataGroupInstancesActivity, vsactivity.DeleteMetadataGroupParams{
		VXID:  params.VXID,
		Group: vscommon.GroupStlSubtitle,
	}).Wait(ctx)
	if err != nil {
		return fmt.Errorf("removing existing %s metadata failed: %w", vscommon.GroupStlSubtitle, err)
	}

	return wfutils.Execute(ctx, activities.Vidispine.ImportFileAsSidecarActivity, vsactivity.ImportSubtitleAsSidecarParams{
		AssetID:  params.VXID,
		FilePath: params.FilePath,
		Language: params.Language,
	}).Wait(ctx)
}
