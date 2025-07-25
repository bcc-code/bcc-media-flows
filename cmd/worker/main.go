package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"reflect"
	"runtime"
	"strconv"
	"time"

	"github.com/bcc-code/bcc-media-flows/analytics"
	"github.com/bcc-code/bcc-media-flows/services/directus"
	"github.com/bcc-code/bcc-media-flows/services/notion"
	wfutils "github.com/bcc-code/bcc-media-flows/utils/workflows"
	miscworkflows "github.com/bcc-code/bcc-media-flows/workflows/misc"
	"github.com/joho/godotenv"

	"github.com/bcc-code/bcc-media-flows/services/rclone"
	"github.com/bcc-code/bcc-media-flows/workflows"

	"github.com/bcc-code/bcc-media-flows/activities"
	batonactivities "github.com/bcc-code/bcc-media-flows/activities/baton"
	"github.com/bcc-code/bcc-media-flows/activities/cantemo"
	"github.com/bcc-code/bcc-media-flows/environment"
	"github.com/teamwork/reload"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/interceptor"

	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"

	selfupdate "github.com/creativeprojects/go-selfupdate"
)

var utilActivities = []any{
	batonactivities.QC,
	cantemo.AddRelation,
	cantemo.RenameFile,
	cantemo.MoveFileWait,
	cantemo.GetTaskInfo,
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
	err := godotenv.Load(".env")
	if err == nil {
		fmt.Println("Env file loaded")
	}

	err = update(Version)
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

	c, err := client.Dial(client.Options{
		HostPort:  os.Getenv("TEMPORAL_HOST_PORT"),
		Namespace: os.Getenv("TEMPORAL_NAMESPACE"),
	})

	if err != nil {
		panic(err)
	}

	defer c.Close()

	analytics.Init(analytics.Config{
		WriteKey:  os.Getenv("RUDDERSTACK_WRITE_KEY"),
		DataPlane: os.Getenv("RUDDERSTACK_DATA_PLANE_URL"),
		Verbose:   false,
	})

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

	if os.Getenv("RCLONE_PASSWORD") != "" {
		go rclone.StartFileTransferQueue()
	}

	registerWorker(c, environment.GetQueue(), workerOptions)
}

func registerWorker(c client.Client, queue string, options worker.Options) {
	w := worker.New(c, queue, options)

	directusBaseURL := os.Getenv("DIRECTUS_BASE_URL")
	directusAPIKey := os.Getenv("DIRECTUS_API_KEY")
	directusClient := directus.NewClient(directusBaseURL, directusAPIKey)
	activities.Directus = &activities.DirectusActivities{
		Client:         directusClient,
		ShortsFolderID: os.Getenv("DIRECTUS_SHORTS_FOLDER_ID"),
	}

	notionAPIKey := os.Getenv("NOTION_API_KEY")
	notionClient, err := notion.NewClient(notionAPIKey)
	if err != nil {
		log.Printf("Error creating notion client: %v", err)
	}
	activities.Notion = &activities.NotionActivities{
		Client:           notionClient,
		ShortsDatabaseID: os.Getenv("NOTION_SHORTS_DATABASE_ID"),
	}

	switch queue {
	case environment.QueueDebug:
		registerActivitiesInStruct(w, activities.Util)

		for _, a := range utilActivities {
			w.RegisterActivity(a)
		}

		registerActivitiesInStruct(w, activities.Vidispine)

		registerActivitiesInStruct(w, activities.Platform)

		registerActivitiesInStruct(w, activities.Video)

		registerActivitiesInStruct(w, activities.Audio)

		registerActivitiesInStruct(w, activities.Directus)

		registerActivitiesInStruct(w, activities.Notion)

		for _, wf := range workflows.WorkerWorkflows {
			w.RegisterWorkflow(wf)
		}
	case environment.QueueLowPriority:
		fallthrough
	case environment.QueueWorker:
		registerActivitiesInStruct(w, activities.Util)

		for _, a := range utilActivities {
			w.RegisterActivity(a)
		}

		registerActivitiesInStruct(w, activities.Platform)
		registerActivitiesInStruct(w, activities.Vidispine)
		registerActivitiesInStruct(w, activities.Directus)
		registerActivitiesInStruct(w, activities.Notion)

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
	err = w.Run(worker.InterruptCh())

	log.Printf("Worker finished: %v", err)

}

func update(version string) error {
	if version == "development" {
		return nil
	}

	// Prevent worker from restarting if there are activities executing
	wfutils.ActivityWG.Wait()

	ctx := context.Background()

	latest, found, err := selfupdate.DetectLatest(ctx, selfupdate.ParseSlug("bcc-code/bcc-media-flows"))
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

	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("could not locate executable path")
	}
	if err := selfupdate.UpdateTo(ctx, latest.AssetURL, latest.AssetName, exe); err != nil {
		return fmt.Errorf("error occurred while updating binary: %w", err)
	}
	log.Printf("Successfully updated to version %s", latest.Version())
	reload.Exec()
	return nil
}
