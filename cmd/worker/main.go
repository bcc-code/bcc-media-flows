package main

import (
	"github.com/bcc-code/bccm-flows/workflows"
	"log"
	"os"
	"time"

	atranscribe "github.com/bcc-code/bccm-flows/activities/transcribe"
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

	workerOptions := worker.Options{
		DeadlockDetectionTimeout:           time.Hour * 3,
		DisableRegistrationAliasing:        true, // Recommended according to readme, default false for backwards compatibility
		EnableSessionWorker:                true,
		Identity:                           "worker",
		LocalActivityWorkerOnly:            false,
		MaxConcurrentActivityExecutionSize: 100, // Doesn't make sense to have more than one activity running at a time
	}

	w := worker.New(c, "worker", workerOptions)

	w.RegisterWorkflow(workflows.TranscribeFile)
	w.RegisterWorkflow(workflows.TranscribeVX)
	w.RegisterActivity(atranscribe.TranscribeActivity)
	w.RegisterActivity(vidispine.GetFileFromVXActivity)
	w.RegisterActivity(vidispine.ImportFileAsShapeActivity)

	err = w.Run(worker.InterruptCh())
	log.Printf("Worker finished: %v", err)
}
