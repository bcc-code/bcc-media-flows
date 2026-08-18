package activities

import (
	cantemoactivities "github.com/bcc-code/bcc-media-flows/activities/cantemo"
	"github.com/bcc-code/bcc-media-flows/services/subtrans"
	"github.com/bcc-code/bcc-media-flows/services/vidispine"
	"reflect"
	"runtime"
	"strings"

	platform_activities "github.com/bcc-code/bcc-media-flows/activities/platform"
	vsactivity "github.com/bcc-code/bcc-media-flows/activities/vidispine"
	"github.com/bcc-code/bcc-media-flows/environment"
)

func GetMethodNames(of any) []string {
	v := reflect.TypeOf(of)
	var activities []string
	for i := 0; i < v.NumMethod(); i++ {
		activities = append(activities, v.Method(i).Name)
	}
	return activities
}

type AudioActivities struct{}

var Audio = AudioActivities{}

type VideoActivities struct{}

var Video = VideoActivities{}

type UtilActivities struct {
	Vidispine vidispine.Client
	Subtrans  *subtrans.Client
}

// Util is replaced at boot with clients built from the configuration.
var Util = &UtilActivities{}

type LiveActivities struct{}

var Live = LiveActivities{}

var Vidispine = vsactivity.Vidispine

var Cantemo = cantemoactivities.Cantemo

var Platform = platform_activities.PlatformActivities

func getFunctionName(i any) string {
	if fullName, ok := i.(string); ok {
		return fullName
	}
	fullName := runtime.FuncForPC(reflect.ValueOf(i).Pointer()).Name()
	elements := strings.Split(fullName, ".")
	shortName := elements[len(elements)-1]
	return strings.TrimSuffix(shortName, "-fm")
}

// activityQueues maps an activity's registered name to the queue whose workers
// have what it needs — ffmpeg for the transcode and audio queues, the live
// ingest machine for live. Everything else runs on the worker queue.
//
// The name is the short method name, which is also what
// registerActivitiesInStruct registers the activity under, so the two agree by
// construction.
var activityQueues = buildActivityQueues()

func buildActivityQueues() map[string]func() string {
	queues := map[string]func() string{}

	add := func(activityStruct any, queue func() string) {
		for _, name := range GetMethodNames(activityStruct) {
			if _, taken := queues[name]; taken {
				// Two structs claiming one name would make routing depend on the
				// order of this function, and Temporal would refuse to register the
				// second one anyway: the worker runs with
				// DisableRegistrationAliasing, and the debug queue registers every
				// struct. Failing at startup beats activities queueing where nothing
				// is listening.
				panic("activity " + name + " is defined on more than one activity struct")
			}
			queues[name] = queue
		}
	}

	add(Audio, environment.GetAudioQueue)
	add(Video, environment.GetTranscodeQueue)
	add(Live, environment.GetLiveIngestQueue)

	return queues
}

// GetQueueForActivity detects which queue the activity belongs in, else returns the worker queue.
// Used to execute the activity where the required dependencies are available.
// For example ffmpeg activities has to be executed in either the Transcode queue or Audio queue where we know ffmpeg is installed on the workers.
//
// The fallback is silent by necessity — the name of an activity says nothing
// about what it needs — so an ffmpeg activity hung on the wrong struct lands on
// the worker queue and fails there as an opaque timeout. TestEveryFFmpegActivityIsOnAnFFmpegQueue
// is what catches that.
func GetQueueForActivity(activity any) string {
	if queue, ok := activityQueues[getFunctionName(activity)]; ok {
		return queue()
	}
	return environment.GetWorkerQueue()
}
