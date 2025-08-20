package workflows

import (
	"github.com/bcc-code/bcc-media-flows/workflows/export"
	ingestworkflows "github.com/bcc-code/bcc-media-flows/workflows/ingest"
	miscworkflows "github.com/bcc-code/bcc-media-flows/workflows/misc"
	"github.com/bcc-code/bcc-media-flows/workflows/scheduled"
	"github.com/bcc-code/bcc-media-flows/workflows/vb_export"
)

var TriggerableWorkflows = []any{
	export.VXExport,
	vb_export.VBExport,
	ingestworkflows.BmmIngestUpload,
	miscworkflows.TranscodePreviewVX,
	miscworkflows.TranscodePreviewFile,
	miscworkflows.TranscribeFile,
	miscworkflows.TranscribeVX,
	miscworkflows.HandleMultitrackFile,
	export.ExportTimedMetadata,
	miscworkflows.ImportSubtitlesFromSubtrans,
	miscworkflows.MergeAndImportSubtitlesFromCSV,
	miscworkflows.UpdateAssetRelations,
	miscworkflows.NormalizeAudioLevelWorkflow,
	miscworkflows.MoveMBFile,
	miscworkflows.MoveFilesWorkerFlow,
	scheduled.CleanupTemp,
	scheduled.MediabankenPurgeTrash,
}

var WorkerWorkflows = []any{
	miscworkflows.TranscodePreviewVX,
	miscworkflows.TranscodePreviewFile,
	miscworkflows.TranscribeFile,
	miscworkflows.TranscribeVX,
	miscworkflows.WatchFolderTranscode,
	miscworkflows.HandleMultitrackFile,
	miscworkflows.MoveMBFile,
	miscworkflows.MoveFilesWorkerFlow,
	ingestworkflows.BmmIngestUpload,
	export.VXExport,
	export.VXExportToVOD,
	export.VXExportToPlayout,
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
	ingestworkflows.Asset,
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
	vb_export.VBExport,
	vb_export.VBExportToAbekas,
	vb_export.VBExportToBStage,
	vb_export.VBExportToGfx,
	vb_export.VBExportToHippo,
	vb_export.VBExportToDubbing,
	vb_export.VBExportToHyperdeck,
	scheduled.CleanupTemp,
	scheduled.MediabankenPurgeTrash,
}
