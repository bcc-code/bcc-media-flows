package main

import (
	batonactivities "github.com/bcc-code/bccm-flows/activities/baton"
	"github.com/bcc-code/bccm-flows/workflows/ingest"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/bcc-code/bccm-flows/workflows/export"

	"github.com/bcc-code/bccm-flows/activities"
	"github.com/bcc-code/bccm-flows/common"
	"github.com/bcc-code/bccm-flows/utils"
	"github.com/bcc-code/bccm-flows/workflows"

	"github.com/bcc-code/bccm-flows/activities/vidispine"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
)

var utilActivities = []any{
	activities.MoveFile,
	activities.CreateFolder,
	activities.WriteFile,
	activities.ReadFile,
	activities.ListFiles,
	activities.DeletePath,
	activities.StandardizeFileName,
	activities.GetSubtitlesActivity,
	batonactivities.QC,
}

var vidispineActivities = []any{
	vidispine.GetFileFromVXActivity,
	vidispine.ImportFileAsShapeActivity,
	vidispine.ImportFileAsSidecarActivity,
	vidispine.CreatePlaceholderActivity,
	vidispine.SetVXMetadataFieldActivity,
	vidispine.GetExportDataActivity,
	vidispine.GetChapterDataActivity,
	vidispine.CreateThumbnailsActivity,
	vidispine.WaitForJobCompletion,
	vidispine.JobCompleteOrErr,
	activities.GetSubtransIDActivity,
}

var transcodeActivities = activities.GetVideoTranscodeActivities()

var audioTranscodeActivities = activities.GetAudioTranscodeActivities()

var workerWorkflows = []any{
	workflows.TranscodePreviewVX,
	workflows.TranscodePreviewFile,
	workflows.TranscribeFile,
	workflows.TranscribeVX,
	workflows.WatchFolderTranscode,
	export.VXExport,
	export.VXExportToVOD,
	export.VXExportToPlayout,
	export.MergeExportData,
	export.MuxFiles,
	export.PrepareFiles,
	export.VXExportToBMM,
	workflows.ExecuteFFmpeg,
	workflows.ImportSubtitlesFromSubtrans,
	ingest.Asset,
	ingest.RawMaterial,
	ingest.VBMaster,
	workflows.NormalizeAudioLevelWorkflow,
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

	registerWorker(c, utils.GetQueue(), workerOptions)
}

func registerWorker(c client.Client, queue string, options worker.Options) {
	w := worker.New(c, queue, options)

	switch queue {
	case common.QueueDebug:
		w.RegisterActivity(activities.Transcribe)
		w.RegisterActivity(activities.RcloneCopyDir)
		w.RegisterActivity(activities.RcloneMoveFile)
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
	case common.QueueWorker:
		w.RegisterActivity(activities.Transcribe)
		w.RegisterActivity(activities.RcloneCopyDir)
		w.RegisterActivity(activities.RcloneMoveFile)
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
	case common.QueueTranscode:
		for _, a := range transcodeActivities {
			w.RegisterActivity(a)
		}
	case common.QueueAudio:
		for _, a := range audioTranscodeActivities {
			w.RegisterActivity(a)
		}
	}

	err := w.Run(worker.InterruptCh())

	log.Printf("Worker finished: %v", err)
}
