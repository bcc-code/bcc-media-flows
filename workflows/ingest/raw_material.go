package ingestworkflows

import (
	"fmt"
	"github.com/bcc-code/bcc-media-flows/services/rclone"
	"strings"

	"github.com/bcc-code/bcc-media-flows/activities"
	vsactivity "github.com/bcc-code/bcc-media-flows/activities/vidispine"
	"github.com/bcc-code/bcc-media-flows/paths"
	"github.com/bcc-code/bcc-media-flows/services/ffmpeg"
	"github.com/bcc-code/bcc-media-flows/services/ingest"
	"github.com/bcc-code/bcc-media-flows/utils"
	wfutils "github.com/bcc-code/bcc-media-flows/utils/workflows"
	miscworkflows "github.com/bcc-code/bcc-media-flows/workflows/misc"
	"go.temporal.io/sdk/workflow"
)

type RawMaterialFormParams struct {
	OrderForm OrderForm
	Targets   []string
	Metadata  *ingest.Metadata
	Directory paths.Path
}

func RawMaterialForm(ctx workflow.Context, params RawMaterialFormParams) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting RawMaterial workflow")

	ctx = workflow.WithActivityOptions(ctx, wfutils.GetDefaultActivityOptions())

	if params.OrderForm != OrderFormRawMaterial {
		return fmt.Errorf("invalid order form: %s", params.OrderForm)
	}

	originalFiles, err := wfutils.ListFiles(ctx, params.Directory)
	if err != nil {
		if nerr := notifyImportFailed(ctx, params.Targets, params.Metadata.JobProperty.JobID, originalFiles, err); nerr != nil {
			logger.Error("Failed to notify about import failure", "error", nerr)
		}
		return err
	}

	fileByAssetID, err := RawMaterial(ctx, RawMaterialParams{
		FilesToIngest:    originalFiles,
		DeliveryMetadata: params.Metadata,
		Language:         params.Metadata.JobProperty.Language,
		Recipients:       params.Targets,
	})
	if err != nil {
		if nerr := notifyImportFailed(ctx, params.Targets, params.Metadata.JobProperty.JobID, originalFiles, err); nerr != nil {
			logger.Error("Failed to notify about import failure", "error", nerr)
		}
		return err
	}

	err = notifyImportCompleted(ctx, params.Targets, params.Metadata.JobProperty.JobID, fileByAssetID)
	if err != nil {
		return err
	}

	return nil
}

type RawMaterialParams struct {
	FilesToIngest    paths.Files
	DeliveryMetadata *ingest.Metadata
	Language         string
	// Recipients are the uploaders, who get the QC report for the video files.
	// Without any the QC still runs, but nobody is mailed.
	Recipients []string
}

func RawMaterial(ctx workflow.Context, params RawMaterialParams) (map[string]paths.Path, error) {
	ctx = workflow.WithActivityOptions(ctx, wfutils.GetDefaultActivityOptions())

	outputDir, err := wfutils.GetWorkflowRawOutputFolder(ctx)
	if err != nil {
		return nil, err
	}

	files := []paths.Path{}
	for _, f := range params.FilesToIngest {
		if !utils.ValidRawFilename(f.Local()) {
			return nil, fmt.Errorf("invalid filename: %s", f)
		}

		newFileName := strings.ReplaceAll(f.Base(), " ", "_")
		newPath := outputDir.Append(newFileName)
		err = wfutils.MoveFile(ctx, f, newPath, rclone.PriorityNormal)
		if err != nil {
			return nil, err
		}

		files = append(files, newPath)
	}

	var fileByAssetID = map[string]paths.Path{}
	var mediaAnalyzeTasks = map[string]wfutils.Task[*ffmpeg.StreamInfo]{}
	var importResults = map[string]*ImportTagResult{}

	imported := map[string]paths.Path{}
	for _, file := range files {
		var result *ImportTagResult
		result, err = ImportFileAsTag(ctx, "original", file, file.Base())
		if err != nil {
			return imported, err
		}

		imported[result.AssetID] = file

		if params.DeliveryMetadata != nil {
			err = addMetaTags(ctx, result.AssetID, params.DeliveryMetadata)
			if err != nil {
				return imported, err
			}
		}

		fileByAssetID[result.AssetID] = file
		importResults[result.AssetID] = result

		if utils.IsMedia(file.Local()) {
			mediaAnalyzeTasks[result.AssetID] = wfutils.Execute(ctx, activities.Audio.AnalyzeFile, activities.AnalyzeFileParams{
				FilePath: file,
			})
		}
	}

	mediaAssetIDs, err := wfutils.GetMapKeysSafely(ctx, mediaAnalyzeTasks)
	if err != nil {
		return imported, err
	}

	audioAssetIDs := []string{}
	previewAssetIDs := []string{}
	var qcFiles []miscworkflows.QScanRawImportFile

	for _, id := range mediaAssetIDs {
		task := mediaAnalyzeTasks[id]
		result, err := task.Result(ctx)
		if err != nil {
			return imported, err
		}

		// need to wait for vidispine to import the file before we can create thumbnails
		err = WaitForImportTag(ctx, importResults[id])
		if err != nil {
			return imported, err
		}
		// Only create thumbnails if the file has video
		if result.HasVideo {
			err = wfutils.Execute(ctx, activities.Vidispine.CreateThumbnailsActivity, vsactivity.CreateThumbnailsParams{
				AssetID: id,
			}).Wait(ctx)
			if err != nil {
				return imported, err
			}

			// Only video goes through QC; audio-only files have no template.
			qcFiles = append(qcFiles, miscworkflows.QScanRawImportFile{VXID: id, Path: fileByAssetID[id]})
		}

		if result.HasAudio {
			audioAssetIDs = append(audioAssetIDs, id)
		}

		// TranscodePreviewVX generates both the video and audio previews in a single
		// pass, so run it once per asset. Each id is processed once here, so the list
		// is inherently duplicate-free and deterministic for Temporal replay.
		if result.HasVideo || result.HasAudio {
			previewAssetIDs = append(previewAssetIDs, id)
		}
	}

	startRawImportQC(ctx, qcFiles, params.Recipients)

	if _, err = createPreviewsAsync(ctx, previewAssetIDs); err != nil {
		return imported, err
	}

	err = transcribe(ctx, audioAssetIDs, params.Language)
	return imported, err
}

// startRawImportQC runs QScan on the video files as an abandoned child: it
// neither delays nor fails the import, and mails one report for the upload.
// The child must have started before this returns, or closing the parent
// would drop it.
func startRawImportQC(ctx workflow.Context, files []miscworkflows.QScanRawImportFile, recipients []string) {
	if len(files) == 0 {
		return
	}

	asyncCtx := wfutils.WithChildSearchAttributes(wfutils.WithAbandonChildOptions(ctx), files[0].VXID)
	future := workflow.ExecuteChildWorkflow(asyncCtx, miscworkflows.QScanRawImport, miscworkflows.QScanRawImportInput{
		Files:      files,
		Recipients: recipients,
	})
	if err := future.GetChildWorkflowExecution().Get(ctx, nil); err != nil {
		workflow.GetLogger(ctx).Error("Failed to start QScan workflow for raw import", "vxid", files[0].VXID, "error", err)
	}
}
