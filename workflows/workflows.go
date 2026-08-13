package workflows

import (
	"github.com/bcc-code/bcc-media-flows/workflows/export"
	ingestworkflows "github.com/bcc-code/bcc-media-flows/workflows/ingest"
	miscworkflows "github.com/bcc-code/bcc-media-flows/workflows/misc"
	"github.com/bcc-code/bcc-media-flows/workflows/scheduled"
	"github.com/bcc-code/bcc-media-flows/workflows/vb_export"
)

// WorkerWorkflows is the registry of workflows this worker runs. A workflow
// missing from it cannot be executed.
var WorkerWorkflows = []any{
	miscworkflows.ImportSidecarSubtitle,
	miscworkflows.TranscodePreviewVX,
	miscworkflows.TranscodePreviewFile,
	miscworkflows.CreateThumbnailsVX,
	miscworkflows.TranscodeHAP,
	miscworkflows.TranscribeFile,
	miscworkflows.TranscribeVX,
	miscworkflows.WatchFolderTranscode,
	miscworkflows.HandleMultitrackFile,
	miscworkflows.MoveMBFile,
	miscworkflows.MoveFilesWorkerFlow,
	miscworkflows.CopyFile,
	ingestworkflows.BmmIngestUpload,
	ingestworkflows.BmmTrackMetadata,
	export.VXExport,
	export.VXExportToVOD,
	export.VXExportToXDCAM,
	export.MergeExportData,
	export.VXExportToBMM,
	export.IsilonExport,
	export.ExportTimedMetadata,
	export.BulkExportShorts,
	export.ExportShort,
	export.GenerateShort,
	miscworkflows.ExecuteFFmpeg,
	miscworkflows.ImportSubtitlesFromSubtrans,
	miscworkflows.MergeAndImportSubtitlesFromCSV,
	miscworkflows.UpdateAssetRelations,
	ingestworkflows.AssetJSON,
	ingestworkflows.RawMaterial,
	ingestworkflows.RawMaterialForm,
	ingestworkflows.Masters,
	ingestworkflows.Incremental,
	ingestworkflows.MoveUploadedFiles,
	ingestworkflows.ImportAudioFileFromReaper,
	ingestworkflows.ExtractAudioFromMU1MU2,
	ingestworkflows.IngestSyncFix,
	ingestworkflows.Multitrack,
	ingestworkflows.ImportSubtitles,
	miscworkflows.NormalizeAudioLevelWorkflow,
	miscworkflows.FixDurationVX,
	vb_export.VBExport,
	vb_export.VBExportToAbekas,
	vb_export.VBExportToRawAbekas,
	vb_export.VBExportToBStage,
	vb_export.VBExportToGfx,
	vb_export.VBExportToHippoV2,
	vb_export.VBExportToHippoHap,
	vb_export.VBExportToDubbing,
	vb_export.VBExportToHyperdeck,
	vb_export.VBExportToXDCAM,
	vb_export.VBExportToCasparCG,
	scheduled.CleanupTemp,
	scheduled.MediabankenPurgeTrash,
	// Massive.app import workflow
	miscworkflows.MASVImport,
}
