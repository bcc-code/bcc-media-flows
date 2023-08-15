package main

import (
	"github.com/bcc-code/bccm-flows/activities"
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
		queue = "worker"
	}
	w := worker.New(c, queue, workerOptions)

	w.RegisterWorkflow(workflows.TranscribeFile)
	w.RegisterWorkflow(workflows.TranscribeVX)
	w.RegisterActivity(activities.Transcribe)
	w.RegisterActivity(vidispine.GetFileFromVXActivity)
	w.RegisterActivity(vidispine.ImportFileAsShapeActivity)

	err = w.Run(worker.InterruptCh())
	log.Printf("Worker finished: %v", err)
}
