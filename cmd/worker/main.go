package main

import (
	"log"
	"os"
	"strconv"
	"time"

	batonactivities "github.com/bcc-code/bcc-media-flows/activities/baton"
	"github.com/bcc-code/bcc-media-flows/activities/cantemo"
	vsactivity "github.com/bcc-code/bcc-media-flows/activities/vidispine"
	"github.com/bcc-code/bcc-media-flows/environment"
	ingestworkflows "github.com/bcc-code/bcc-media-flows/workflows/ingest"
	"github.com/bcc-code/bcc-media-flows/workflows/scheduled"
	"github.com/bcc-code/bcc-media-flows/workflows/vb_export"
	"github.com/bcc-code/bcc-media-flows/workflows/webhooks"

	"github.com/bcc-code/bcc-media-flows/workflows/export"

	"github.com/bcc-code/bcc-media-flows/activities"
	"github.com/bcc-code/bcc-media-flows/workflows"

	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
)

var utilActivities = []any{
	activities.MoveFile,
	activities.CreateFolder,
	activities.WriteFile,
	activities.ReadFile,
	activities.ListFiles,
	activities.CopyFile,
	activities.WaitForFile,
	activities.DeletePath,
	activities.StandardizeFileName,
	activities.GetSubtitlesActivity,
	activities.MergeTranscriptJSON,
	batonactivities.QC,
	activities.FtpPlayoutRename,
	activities.NotifySimple,
	activities.NotifyImportCompleted,
	activities.RsyncIncrementalCopy,
	activities.StartReaper,
	activities.StopReaper,
	activities.ListReaperFiles,
	activities.DeleteEmptyDirectories,
	activities.DeleteOldFiles,
}

var vidispineActivities = []any{
	vsactivity.GetFileFromVXActivity,
	vsactivity.ImportFileAsShapeActivity,
	vsactivity.ImportFileAsSidecarActivity,
	vsactivity.CreatePlaceholderActivity,
	vsactivity.SetVXMetadataFieldActivity,
	vsactivity.GetExportDataActivity,
	vsactivity.GetChapterDataActivity,
	vsactivity.CreateThumbnailsActivity,
	vsactivity.WaitForJobCompletion,
	vsactivity.JobCompleteOrErr,
	vsactivity.AddFileToPlaceholder,
	vsactivity.CloseFile,
	activities.GetSubtransIDActivity,
	cantemo.AddRelation,
}

var transcodeActivities = activities.GetVideoTranscodeActivities()

var audioTranscodeActivities = activities.GetAudioTranscodeActivities()

var workerWorkflows = []any{
	workflows.TranscodePreviewVX,
	workflows.TranscodePreviewFile,
	workflows.TranscribeFile,
	workflows.TranscribeVX,
	workflows.WatchFolderTranscode,
	workflows.HandleMultitrackFile,
	webhooks.WebHook,
	webhooks.BmmSimpleUpload,
	export.VXExport,
	export.VXExportToVOD,
	export.VXExportToPlayout,
	export.MergeExportData,
	export.VXExportToBMM,
	workflows.ExecuteFFmpeg,
	workflows.ImportSubtitlesFromSubtrans,
	ingestworkflows.Asset,
	ingestworkflows.RawMaterial,
	ingestworkflows.Masters,
	ingestworkflows.Incremental,
	ingestworkflows.MoveUploadedFiles,
	ingestworkflows.ImportAudioFileFromReaper,
	workflows.NormalizeAudioLevelWorkflow,
	vb_export.VBExport,
	vb_export.VBExportToAbekas,
	vb_export.VBExportToBStage,
	vb_export.VBExportToGfx,
	vb_export.VBExportToHippo,
	scheduled.CleanupTemp,
}

func main() {
	c, err := client.Dial(client.Options{
		HostPort:  os.Getenv("TEMPORAL_HOST_PORT"),
		Namespace: os.Getenv("TEMPORAL_NAMESPACE"),
	})

	if err != nil {
		panic(err)
	}

	defer c.Close()

	identity := os.Getenv("IDENTITY")
	if identity == "" {
		identity = "worker"
	}

	activityCountString := os.Getenv("ACTIVITY_COUNT")
	if activityCountString == "" {
		activityCountString = "5"
	}

	activityCount, err := strconv.Atoi(activityCountString)
	if err != nil {
		panic(err)
	}

	workerOptions := worker.Options{
		DeadlockDetectionTimeout:           time.Hour * 3,
		DisableRegistrationAliasing:        true, // Recommended according to readme, default false for backwards compatibility
		EnableSessionWorker:                true,
		Identity:                           identity,
		LocalActivityWorkerOnly:            false,
		MaxConcurrentActivityExecutionSize: activityCount, // Doesn't make sense to have more than one activity running at a time
	}

	registerWorker(c, environment.GetQueue(), workerOptions)
}

func registerWorker(c client.Client, queue string, options worker.Options) {
	w := worker.New(c, queue, options)

	switch queue {
	case environment.QueueDebug:
		w.RegisterActivity(activities.Transcribe)
		w.RegisterActivity(activities.RcloneCopyDir)
		w.RegisterActivity(activities.RcloneMoveFile)
		w.RegisterActivity(activities.RcloneCopyFile)
		w.RegisterActivity(activities.PubsubPublish)

		for _, a := range utilActivities {
			w.RegisterActivity(a)
		}

		for _, a := range vidispineActivities {
			w.RegisterActivity(a)
		}

		for _, a := range transcodeActivities {
			w.RegisterActivity(a)
		}

		for _, a := range audioTranscodeActivities {
			w.RegisterActivity(a)
		}

		for _, wf := range workerWorkflows {
			w.RegisterWorkflow(wf)
		}
	case environment.QueueLowPriority:
		fallthrough
	case environment.QueueWorker:
		w.RegisterActivity(activities.Transcribe)
		w.RegisterActivity(activities.RcloneCopyDir)
		w.RegisterActivity(activities.RcloneMoveFile)
		w.RegisterActivity(activities.RcloneCopyFile)
		w.RegisterActivity(activities.PubsubPublish)

		for _, a := range utilActivities {
			w.RegisterActivity(a)
		}

		for _, a := range vidispineActivities {
			w.RegisterActivity(a)
		}

		for _, wf := range workerWorkflows {
			w.RegisterWorkflow(wf)
		}
	case environment.QueueTranscode:
		for _, a := range transcodeActivities {
			w.RegisterActivity(a)
		}
	case environment.QueueAudio:
		for _, a := range audioTranscodeActivities {
			w.RegisterActivity(a)
		}
	}

	err := w.Run(worker.InterruptCh())

	log.Printf("Worker finished: %v", err)
}
