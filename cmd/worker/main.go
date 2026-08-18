package main

import (
	"context"
	"fmt"
	"github.com/bcc-code/bcc-media-flows/internal/bootstrap"
	cantemo "github.com/bcc-code/bcc-media-flows/services/cantemo"
	"github.com/bcc-code/bcc-media-flows/services/subtrans"
	"github.com/bcc-code/bcc-media-flows/services/vidispine/vsapi"
	"log"
	"os"
	"os/exec"
	"reflect"
	"runtime"
	"time"

	"github.com/bcc-code/bcc-media-flows/analytics"
	"github.com/bcc-code/bcc-media-flows/services/clickup"
	"github.com/bcc-code/bcc-media-flows/services/directus"
	"github.com/bcc-code/bcc-media-flows/services/vizualizer"
	wfutils "github.com/bcc-code/bcc-media-flows/utils/workflows"
	miscworkflows "github.com/bcc-code/bcc-media-flows/workflows/misc"

	"github.com/bcc-code/bcc-media-flows/services/rclone"
	"github.com/bcc-code/bcc-media-flows/workflows"

	"github.com/bcc-code/bcc-media-flows/activities"
	"github.com/bcc-code/bcc-media-flows/environment"
	"github.com/teamwork/reload"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/interceptor"

	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"

	selfupdate "github.com/creativeprojects/go-selfupdate"
)

// utilActivities is a function, not a var: a method value captures the receiver, and
// at package-init time the clients are not built yet.
func utilActivities() []any {
	return []any{
		activities.Cantemo.AddRelation,
		activities.Cantemo.RenameFile,
		activities.Cantemo.MoveFileWait,
		activities.Cantemo.GetTaskInfo,
	}
}

// registerActivitiesInStruct registers all methods in a struct as activities
func registerActivitiesInStruct(w worker.Worker, activityStruct any) {
	v := reflect.ValueOf(activityStruct)
	t := reflect.TypeOf(activityStruct)
	for i := 0; i < t.NumMethod(); i++ {
		method := t.Method(i)
		f := v.MethodByName(method.Name)
		opts := activity.RegisterOptions{
			Name: method.Name,
		}
		w.RegisterActivityWithOptions(f.Interface(), opts)
	}
}

var analyticsSvc *analytics.Service

func GetAnalyticsService() *analytics.Service {
	return analyticsSvc
}

var Version = "development"

func main() {
	bootstrap.LoadEnv()
	environment.Load()
	environment.WarnMissing(environment.RequiredByWorker)

	err := update(Version)
	if err != nil {
		panic(err)
	}

	go func() {
		for {
			time.Sleep(5 * time.Minute)
			err := update(Version)
			if err != nil {
				log.Printf("Error updating worker: %v", err)
			}
		}
	}()

	c, err := bootstrap.TemporalClient()
	if err != nil {
		panic(err)
	}

	defer c.Close()

	err = wfutils.EnsureSearchAttributes(context.Background(), c)
	if err != nil {
		log.Printf("Error registering search attributes: %v", err)
	}

	analytics.Init(analytics.Config{
		WriteKey:  environment.Get().Rudderstack.WriteKey(),
		DataPlane: environment.Get().Rudderstack.DataPlaneURL(),
		Verbose:   false,
	})

	identity := bootstrap.Identity()

	activityCount := environment.Get().ActivityCount

	if environment.GetQueue() == environment.QueueAudio {
		// Test if the libfdk_aac encoder is available
		cmd := exec.Command("ffmpeg", "-xerror",
			"-f", "lavfi", "-xerror",
			"-i", "sine=frequency=1000:duration=0.1",
			"-c:a", "libfdk_aac",
			"-f", "null", "-")

		err := cmd.Run()
		if err != nil {
			panic(err)
		}

		if cmd.ProcessState.ExitCode() != 0 {
			panic("audio worker must support ffmpeg with libfdk_aac")
		}
	}

	ctx := context.Background()

	workerOptions := worker.Options{
		DeadlockDetectionTimeout:           time.Hour * 3,
		DisableRegistrationAliasing:        true, // Recommended according to readme, default false for backwards compatibility
		EnableSessionWorker:                true,
		Identity:                           identity,
		LocalActivityWorkerOnly:            false,
		MaxConcurrentActivityExecutionSize: activityCount, // Doesn't make sense to have more than one activity running at a time
		BackgroundActivityContext:          context.WithValue(ctx, miscworkflows.ClientContextKey, c),
		Interceptors: []interceptor.WorkerInterceptor{
			&wfutils.AnalyticsWorkerInterceptor{},
		},
	}

	if environment.Get().Rclone.Password() != "" {
		go rclone.StartFileTransferQueue()
	}

	buildClients(environment.Get())

	registerWorker(c, environment.GetQueue(), workerOptions)
}

// buildClients constructs every service client once, from the configuration, and hands
// them to the activities that use them. Nothing reaches for a client later.
func buildClients(cfg *environment.Config) {
	vsClient := vsapi.NewClient(cfg.Vidispine)
	cantemoClient := cantemo.NewClient(cfg.Cantemo)

	activities.Vidispine.Client = vsClient
	activities.Cantemo.Client = cantemoClient

	activities.Platform.Vidispine = vsClient
	activities.Platform.Cantemo = cantemoClient

	activities.Util.Vidispine = vsClient
	activities.Util.Subtrans = subtrans.NewClient(cfg.Subtrans)

	activities.Directus = &activities.DirectusActivities{
		Client:         directus.NewClient(cfg.Directus),
		ShortsFolderID: cfg.Directus.ShortsFolderID(),
	}

	clickUpClient, err := clickup.NewClient(cfg.ClickUp)
	if err != nil {
		log.Printf("Error creating ClickUp client: %v", err)
	}
	activities.ClickUp = &activities.ClickUpActivities{Client: clickUpClient}

	vizClient, err := vizualizer.NewClient(cfg.Services)
	if err != nil {
		log.Printf("Error creating vizualizer client: %v", err)
	}
	activities.Vizualizer = &activities.VizualizerActivities{Client: vizClient}
}

func registerWorker(c client.Client, queue string, options worker.Options) {
	w := worker.New(c, queue, options)

	switch queue {
	case environment.QueueDebug:
		registerActivitiesInStruct(w, activities.Util)

		for _, a := range utilActivities() {
			w.RegisterActivity(a)
		}

		registerActivitiesInStruct(w, activities.Vidispine)

		registerActivitiesInStruct(w, activities.Platform)

		registerActivitiesInStruct(w, activities.Video)

		registerActivitiesInStruct(w, activities.Audio)

		registerActivitiesInStruct(w, activities.Directus)

		registerActivitiesInStruct(w, activities.ClickUp)

		registerActivitiesInStruct(w, activities.Vizualizer)

		for _, wf := range workflows.WorkerWorkflows {
			w.RegisterWorkflow(wf)
		}
	case environment.QueueLowPriority:
		fallthrough
	case environment.QueueWorker:
		registerActivitiesInStruct(w, activities.Util)

		for _, a := range utilActivities() {
			w.RegisterActivity(a)
		}

		registerActivitiesInStruct(w, activities.Platform)
		registerActivitiesInStruct(w, activities.Vidispine)
		registerActivitiesInStruct(w, activities.Directus)
		registerActivitiesInStruct(w, activities.ClickUp)
		registerActivitiesInStruct(w, activities.Vizualizer)

		for _, wf := range workflows.WorkerWorkflows {
			w.RegisterWorkflow(wf)
		}
	case environment.QueueTranscode:
		registerActivitiesInStruct(w, activities.Video)
	case environment.QueueAudio:
		registerActivitiesInStruct(w, activities.Audio)
	case environment.QueueLiveIngest:
		registerActivitiesInStruct(w, activities.Live)

	}

	fmt.Println("STARTING")
	err := w.Run(worker.InterruptCh())

	log.Printf("Worker finished: %v", err)

}

func update(version string) error {
	if version == "development" {
		return nil
	}

	ctx := context.Background()

	// SHAValidator makes the update fail closed: the release must carry a
	// <binary>.sha256 asset whose hash matches what was downloaded, or nothing
	// is installed. publish.yml writes that file next to every binary.
	//
	// This catches a truncated or corrupted download, not a compromised
	// release pipeline — the hash is published by the same job, to the same
	// release, so whoever can write one can write the other. Closing that
	// needs a signature checked against a key the pipeline cannot mint.
	updater, err := selfupdate.NewUpdater(selfupdate.Config{
		Validator: &selfupdate.SHAValidator{},
	})
	if err != nil {
		return fmt.Errorf("could not create updater: %w", err)
	}

	latest, found, err := updater.DetectLatest(ctx, selfupdate.ParseSlug("bcc-code/bcc-media-flows"))
	if err != nil {
		return fmt.Errorf("error occurred while detecting version: %w", err)
	}
	if !found {
		return fmt.Errorf("latest version for %s/%s could not be found from github repository", runtime.GOOS, runtime.GOARCH)
	}

	if latest.LessOrEqual(version) {
		log.Printf("Current version (%s) is the latest", version)
		return nil
	}

	// There is an update to install, and installing it restarts the process.
	// Leave it for the next tick if this worker is in the middle of something;
	// the caller runs this every five minutes.
	if running := wfutils.RunningActivities.Running(); running > 0 {
		log.Printf("Version %s is available, deferring update: %d activities running", latest.Version(), running)
		return nil
	}

	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("could not locate executable path")
	}
	if err := updater.UpdateTo(ctx, latest, exe); err != nil {
		return fmt.Errorf("error occurred while updating binary: %w", err)
	}
	log.Printf("Successfully updated to version %s", latest.Version())
	reload.Exec()
	return nil
}
