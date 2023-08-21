package main

import (
	"github.com/bcc-code/bccm-flows/activities"
	"github.com/bcc-code/bccm-flows/common"
	"github.com/bcc-code/bccm-flows/workflows"
	"log"
	"os"
	"time"

	"github.com/bcc-code/bccm-flows/activities/vidispine"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
)

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

	workerOptions := worker.Options{
		DeadlockDetectionTimeout:           time.Hour * 3,
		DisableRegistrationAliasing:        true, // Recommended according to readme, default false for backwards compatibility
		EnableSessionWorker:                true,
		Identity:                           identity,
		LocalActivityWorkerOnly:            false,
		MaxConcurrentActivityExecutionSize: 100, // Doesn't make sense to have more than one activity running at a time

	}

	queue := os.Getenv("QUEUE")
	if queue == "" {
		queue = common.QueueWorker
	}
	w := worker.New(c, queue, workerOptions)

	switch queue {
	case common.QueueDebug:
		w.RegisterActivity(activities.Transcribe)
		w.RegisterActivity(vidispine.GetFileFromVXActivity)
		w.RegisterActivity(vidispine.ImportFileAsShapeActivity)
		w.RegisterActivity(vidispine.ImportFileAsSidecarActivity)
		w.RegisterActivity(vidispine.SetVXMetadataFieldActivity)
		w.RegisterWorkflow(workflows.TranscodePreviewVX)
		w.RegisterWorkflow(workflows.TranscodePreviewFile)
		w.RegisterWorkflow(workflows.TranscribeFile)
		w.RegisterWorkflow(workflows.TranscribeVX)
		w.RegisterWorkflow(workflows.WatchFolderTranscode)
		w.RegisterActivity(activities.TranscodePreview)
		w.RegisterActivity(activities.TranscodeToProResActivity)
	case common.QueueWorker:
		w.RegisterActivity(activities.Transcribe)
		w.RegisterActivity(vidispine.GetFileFromVXActivity)
		w.RegisterActivity(vidispine.ImportFileAsShapeActivity)
		w.RegisterActivity(vidispine.ImportFileAsSidecarActivity)
		w.RegisterActivity(vidispine.SetVXMetadataFieldActivity)
		w.RegisterWorkflow(workflows.TranscodePreviewVX)
		w.RegisterWorkflow(workflows.TranscodePreviewFile)
		w.RegisterWorkflow(workflows.TranscribeFile)
		w.RegisterWorkflow(workflows.TranscribeVX)
		w.RegisterWorkflow(workflows.WatchFolderTranscode)
	case common.QueueTranscode:
		w.RegisterActivity(activities.TranscodePreview)
		w.RegisterActivity(activities.TranscodeToProResActivity)
	}

	err = w.Run(worker.InterruptCh())
	log.Printf("Worker finished: %v", err)
}
