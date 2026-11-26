package miscworkflows

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/bcc-code/bcc-media-flows/internal/enumjson"
	"github.com/orsinium-labs/enum"

	"github.com/bcc-code/bcc-media-flows/paths"
	"github.com/bcc-code/bcc-media-flows/services/rclone"
	"github.com/bcc-code/bcc-media-flows/services/vidispine/vsapi"

	"github.com/bcc-code/bcc-media-flows/services/telegram"

	"github.com/bcc-code/bcc-media-flows/activities"
	vsactivity "github.com/bcc-code/bcc-media-flows/activities/vidispine"
	"github.com/bcc-code/bcc-media-flows/common"
	wfutils "github.com/bcc-code/bcc-media-flows/utils/workflows"

	"go.temporal.io/sdk/workflow"
)

const transcriptionMetadataFieldName = "portal_mf624761"

// TranscriptionDestination is where TranscribeVX delivers the transcript.
// The zero value means Mediabanken, so inputs written before the field existed,
// and callers that leave it out, behave as they always did.
type TranscriptionDestination enum.Member[string]

var (
	TranscriptionDestinationMediabanken = TranscriptionDestination{Value: "mediabanken"}
	TranscriptionDestinationMediaBrain  = TranscriptionDestination{Value: "mediabrain"}
	TranscriptionDestinations           = enum.New(TranscriptionDestinationMediabanken, TranscriptionDestinationMediaBrain)

	ErrUnknownTranscriptionDestination = errors.New("unknown transcription destination")
)

func (d TranscriptionDestination) String() string { return d.Value }

// IsMediaBrain is false for the zero value, which delivers to Mediabanken.
func (d TranscriptionDestination) IsMediaBrain() bool { return d == TranscriptionDestinationMediaBrain }

//goland:noinspection GoMixedReceiverTypes
func (d TranscriptionDestination) MarshalJSON() ([]byte, error) { return enumjson.Marshal(d) }

// UnmarshalJSON takes null and "" as the zero value; only a named, unknown
// destination is refused.
//
//goland:noinspection GoMixedReceiverTypes
func (d *TranscriptionDestination) UnmarshalJSON(data []byte) error {
	var value *string
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	if value == nil || *value == "" {
		*d = TranscriptionDestination{}
		return nil
	}
	return enumjson.UnmarshalStrict(data, TranscriptionDestinations, d, ErrUnknownTranscriptionDestination)
}

// ParseTranscriptionDestination reads a destination from a form or query value. An
// empty value is the zero value, which delivers to Mediabanken.
func ParseTranscriptionDestination(value string) (TranscriptionDestination, error) {
	if value == "" {
		return TranscriptionDestination{}, nil
	}
	d := TranscriptionDestinations.Parse(value)
	if d == nil {
		return TranscriptionDestination{}, fmt.Errorf("%w: %q", ErrUnknownTranscriptionDestination, value)
	}
	return *d, nil
}

// TranscribeVXInput is the input to TranscribeVX
type TranscribeVXInput struct {
	Language            string
	VXID                string
	NotificationChannel *telegram.Chat
	// Destination is optional. Left out, the transcript goes to Mediabanken.
	Destination TranscriptionDestination `json:",omitzero"`
	// Model is optional. Left out, the transcriber uses its default model.
	Model string `json:",omitempty"`
}

// TranscribeVX is the workflow that transcribes a video
func TranscribeVX(
	ctx workflow.Context,
	params TranscribeVXInput,
) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting TranscribeVX")

	ctx = workflow.WithActivityOptions(ctx, wfutils.GetDefaultActivityOptions())

	shapes, err := wfutils.Execute(ctx, activities.Vidispine.GetFileFromVXActivity, vsactivity.GetFileFromVXParams{
		Tags: []vsapi.ShapeTag{vsapi.ShapeTagLowres, vsapi.ShapeTagLowresWatermarked, vsapi.ShapeTagLowAudio, vsapi.ShapeTagOriginal},
		VXID: params.VXID,
	}).Result(ctx)

	if err != nil {
		return err
	}

	tempFolder, err := wfutils.GetWorkflowTempFolder(ctx)
	if err != nil {
		return err
	}

	prepareResult, err := wfutils.Execute(ctx, activities.Audio.PrepareForTranscription, common.AudioInput{
		Path:            shapes.FilePath,
		DestinationPath: tempFolder,
	}).Result(ctx)
	if err != nil {
		return err
	}

	if !prepareResult.HasAudio {
		return nil
	}

	destinationPath, err := wfutils.GetWorkflowAuxOutputFolder(ctx)
	if err != nil {
		return err
	}

	// The transcriber can't write to Isilon, so it writes to temp and the transcripts
	// are copied to aux. They have to end up there because Vidispine imports a shape
	// by registering the file where it lies, and temp gets cleaned
	transcribeToTemp := workflow.GetVersion(ctx, "transcribe-to-temp", workflow.DefaultVersion, 1) >= 1

	transcribeDestination := destinationPath
	if transcribeToTemp {
		transcribeDestination = tempFolder
	}

	transcriptionJob, err := wfutils.Execute(ctx, activities.Transcriber.Transcribe, activities.TranscribeParams{
		Language:        params.Language,
		File:            *prepareResult.OutputPath,
		DestinationPath: transcribeDestination,
		Model:           params.Model,
	}).Result(ctx)

	if err != nil {
		return err
	}

	if params.Destination.IsMediaBrain() {
		return importTranscriptToMediaBrain(ctx, params, transcriptionJob)
	}

	if transcribeToTemp {
		// Copy rather than move: a retried move fails once the source is gone
		for _, p := range []*paths.Path{&transcriptionJob.JSONPath, &transcriptionJob.SRTPath, &transcriptionJob.TXTPath} {
			*p, err = wfutils.CopyToFolder(ctx, *p, destinationPath, rclone.PriorityNormal)
			if err != nil {
				return fmt.Errorf("copying transcript to aux folder: %w", err)
			}
		}
	}

	importJsonJob := wfutils.Execute(ctx, activities.Vidispine.ImportFileAsShapeActivity,
		vsactivity.ImportFileAsShapeParams{
			AssetID:  params.VXID,
			FilePath: transcriptionJob.JSONPath,
			ShapeTag: vsapi.ShapeTagTranscriptionJSON,
			Replace:  true,
		})

	importSRTJob := wfutils.Execute(ctx, activities.Vidispine.ImportFileAsShapeActivity,
		vsactivity.ImportFileAsShapeParams{
			AssetID:  params.VXID,
			FilePath: transcriptionJob.SRTPath,
			ShapeTag: vsapi.ShapeTagTranscribedSubtitleSRT,
			Replace:  true,
		})

	var errs []error
	importJsonResult, err := importJsonJob.Result(ctx)
	if err != nil {
		errs = append(errs, err)
	}

	err = importSRTJob.Wait(ctx)
	if err != nil {
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		return fmt.Errorf("failed to import transcription files: %w", errors.Join(errs...))
	}

	err = wfutils.WaitForVidispineJob(ctx, importJsonResult.JobID)
	if err != nil {
		return fmt.Errorf("importing of JSON file into Mediabanken failed: %w", err)
	}

	// Hand the sidecar import off rather than awaiting it. It runs as a detached
	// child workflow so it survives this workflow completing — an activity would
	// have been cancelled instead. We wait for the child to START, not to finish,
	// because an ABANDON child initiated in the same workflow task that closes the
	// parent is dropped without ever running.
	sidecarFuture := workflow.ExecuteChildWorkflow(
		wfutils.WithAbandonChildOptions(ctx),
		ImportSidecarSubtitle,
		ImportSidecarSubtitleInput{
			VXID:     params.VXID,
			FilePath: transcriptionJob.SRTPath,
			Language: "no",
		},
	)
	if err := sidecarFuture.GetChildWorkflowExecution().Get(ctx, nil); err != nil {
		return fmt.Errorf("failed to start sidecar subtitle import: %w", err)
	}

	if params.NotificationChannel != nil {
		wfutils.SendTelegramText(ctx, *params.NotificationChannel, fmt.Sprintf("🟦 Transcription import completed for VXID: %s", params.VXID))
	}

	txtValue, err := wfutils.ReadFile(ctx, transcriptionJob.TXTPath)
	if err != nil {
		return err
	}

	return wfutils.Execute(ctx, activities.Vidispine.SetVXMetadataFieldActivity, vsactivity.VXMetadataFieldParams{
		ItemID: params.VXID,
		Key:    transcriptionMetadataFieldName,
		Value:  string(txtValue),
	}).Wait(ctx)
}

// importTranscriptToMediaBrain delivers the SRT, with the words of the JSON, to Media
// Brain in place of every Mediabanken step: no shapes, no sidecar and no metadata field
// in Vidispine.
func importTranscriptToMediaBrain(ctx workflow.Context, params TranscribeVXInput, transcript *activities.TranscribeResponse) error {
	res, err := wfutils.Execute(ctx, activities.MediaBrain.ImportTranscriptToMediaBrain, activities.ImportTranscriptToMediaBrainParams{
		VXID:     params.VXID,
		Language: params.Language,
		SRTPath:  transcript.SRTPath,
		JSONPath: &transcript.JSONPath,
		Model:    transcript.Model,
	}).Result(ctx)
	if err != nil {
		return fmt.Errorf("importing transcript to Media Brain: %w", err)
	}

	workflow.GetLogger(ctx).Info("Imported transcript to Media Brain", "vxid", params.VXID, "asset", res.AssetID, "track", res.TrackID, "revision", res.Revision)

	if params.NotificationChannel != nil {
		wfutils.SendTelegramText(ctx, *params.NotificationChannel, fmt.Sprintf("🟦 Transcription imported to Media Brain for VXID: %s", params.VXID))
	}

	return nil
}
