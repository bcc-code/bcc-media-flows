package activities

import (
	"reflect"
	"runtime"
	"strings"

	vsactivity "github.com/bcc-code/bcc-media-flows/activities/vidispine"
	"github.com/bcc-code/bcc-media-flows/environment"
	"github.com/samber/lo"
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

type UtilActivities struct{}

var Util = UtilActivities{}

var Vidispine = vsactivity.Vidispine

func getFunctionName(i any) string {
	if fullName, ok := i.(string); ok {
		return fullName
	}
	fullName := runtime.FuncForPC(reflect.ValueOf(i).Pointer()).Name()
	elements := strings.Split(fullName, ".")
	shortName := elements[len(elements)-1]
	return strings.TrimSuffix(shortName, "-fm")
}

var audioActivities = GetMethodNames(Audio)

var videoActivities = GetMethodNames(Video)

// GetQueueForActivity detects which queue the activity belongs in, else returns the worker queue.
// Used to execute the activity where the required dependencies are available.
// For example ffmpeg activities has to be executed in either the Transcode queue or Audio queue where we know ffmpeg is installed on the workers.
func GetQueueForActivity(activity any) string {
	f := getFunctionName(activity)
	if lo.Contains(audioActivities, f) {
		return environment.GetAudioQueue()
	}
	if lo.Contains(videoActivities, f) {
		return environment.GetTranscodeQueue()
	}
	return environment.GetWorkerQueue()
}
