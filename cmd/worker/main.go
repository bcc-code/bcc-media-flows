package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"reflect"
	"runtime"
	"strconv"
	"time"

	"github.com/bcc-code/bcc-media-flows/services/rclone"

	batonactivities "github.com/bcc-code/bcc-media-flows/activities/baton"
	"github.com/bcc-code/bcc-media-flows/activities/cantemo"
	"github.com/bcc-code/bcc-media-flows/environment"
	ingestworkflows "github.com/bcc-code/bcc-media-flows/workflows/ingest"
	"github.com/bcc-code/bcc-media-flows/workflows/scheduled"
	"github.com/bcc-code/bcc-media-flows/workflows/vb_export"
	"github.com/bcc-code/bcc-media-flows/workflows/webhooks"
	"github.com/teamwork/reload"
	"go.temporal.io/sdk/activity"

	"github.com/bcc-code/bcc-media-flows/workflows/export"

	"github.com/bcc-code/bcc-media-flows/activities"
	"github.com/bcc-code/bcc-media-flows/workflows"

	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"

	selfupdate "github.com/creativeprojects/go-selfupdate"
)

var utilActivities = []any{
	batonactivities.QC,
	cantemo.AddRelation,
}

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
	export.ExportTimedMetadata,
	workflows.ExecuteFFmpeg,
	workflows.ImportSubtitlesFromSubtrans,
	workflows.UpdateAssetRelations,
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
	workflows.NormalizeAudioLevelWorkflow,
	vb_export.VBExport,
	vb_export.VBExportToAbekas,
	vb_export.VBExportToBStage,
	vb_export.VBExportToGfx,
	vb_export.VBExportToHippo,
	vb_export.VBExportToDubbing,
	scheduled.CleanupTemp,
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

var Version = "development"

func main() {
	err := update(Version)
	if err != nil {
		panic(err)
	}

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

	if os.Getenv("RCLONE_PASSWORD") != "" {
		go rclone.StartFileTransferQueue()
	}

	registerWorker(c, environment.GetQueue(), workerOptions)
}

func registerWorker(c client.Client, queue string, options worker.Options) {
	w := worker.New(c, queue, options)

	switch queue {
	case environment.QueueDebug:
		registerActivitiesInStruct(w, activities.Util)

		for _, a := range utilActivities {
			w.RegisterActivity(a)
		}

		registerActivitiesInStruct(w, activities.Vidispine)

		registerActivitiesInStruct(w, activities.Video)

		registerActivitiesInStruct(w, activities.Audio)

		for _, wf := range workerWorkflows {
			w.RegisterWorkflow(wf)
		}
	case environment.QueueLowPriority:
		fallthrough
	case environment.QueueWorker:
		registerActivitiesInStruct(w, activities.Util)

		for _, a := range utilActivities {
			w.RegisterActivity(a)
		}

		registerActivitiesInStruct(w, activities.Vidispine)

		for _, wf := range workerWorkflows {
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
