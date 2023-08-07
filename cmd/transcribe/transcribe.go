package main

import (
	"log"
	"time"

	atranscribe "github.com/bcc-code/bccm-flows/activities/transcribe"
	wtranscribe "github.com/bcc-code/bccm-flows/workflows/transcribe"

	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
)

func main() {
	c, err := client.Dial(client.Options{
		HostPort: client.DefaultHostPort,
	})

	if err != nil {
		panic(err)
	}

	defer c.Close()

	workerOptions := worker.Options{
		DeadlockDetectionTimeout:           time.Hour * 3,
		DisableRegistrationAliasing:        true, // Recommended according to readme, default false for backwards compatibility
		EnableSessionWorker:                true,
		Identity:                           "transcribe-worker",
		LocalActivityWorkerOnly:            false,
		MaxConcurrentActivityExecutionSize: 100, // Doesn't make sense to have more than one activity running at a time
	}

	w := worker.New(c, "transcribe", workerOptions)

	w.RegisterWorkflow(wtranscribe.TranscribeWorkflow)
	w.RegisterActivity(atranscribe.TranscribeActivity)

	err = w.Run(worker.InterruptCh())
	log.Printf("Worker finished: %v", err)
}
