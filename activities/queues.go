package activities

import (
	"reflect"
	"runtime"
	"strings"

	"github.com/bcc-code/bcc-media-flows/environment"

	"github.com/samber/lo"
)

// GetAudioTranscodeActivities returns all activities that should be executed in the Audio queue.
// The workers here have multiple threads and can parallelize audio transcoding tasks (ffmpeg).
func GetAudioTranscodeActivities() []any {
	return []any{
		TranscodeToAudioAac,
		TranscodeToAudioMP3,
		TranscodeToAudioWav,
		TranscodeMux,
		TranscodeMergeAudio,
		AnalyzeEBUR128Activity,
		AdjustAudioLevelActivity,
		AnalyzeFile,
		NormalizeAudioActivity,
		SplitAudioChannels,
		PrependSilence,
		DetectSilence,
		AdjustAudioToVideoStart,
		ExtractAudio,
		TrimFile,
	}
}

// GetVideoTranscodeActivities returns all activities that should be executed in the transcode queue.
// The workers here have multiple threads but only runs one ffmpeg process at a time.
func GetVideoTranscodeActivities() []any {
	return []any{
		TranscodePreview,
		TranscodeToProResActivity,
		TranscodeToAVCIntraActivity,
		TranscodeToH264Activity,
		TranscodeToXDCAMActivity,
		TranscodeMergeVideo,
		TranscodeMergeSubtitles,
		TranscodeToVideoH264,
		TranscodePlayoutMux,
		TranscodeMuxToSimpleMXF,
		ExecuteFFmpeg,
		MultitrackMux,
		GetVideoOffset,
	}
}

func getFunctionName(i any) string {
	if fullName, ok := i.(string); ok {
		return fullName
	}
	fullName := runtime.FuncForPC(reflect.ValueOf(i).Pointer()).Name()
	elements := strings.Split(fullName, ".")
	shortName := elements[len(elements)-1]
	return strings.TrimSuffix(shortName, "-fm")
}

var audioActivities = lo.Map(GetAudioTranscodeActivities(), func(i any, _ int) string {
	return getFunctionName(i)
})

var videoActivities = lo.Map(GetVideoTranscodeActivities(), func(i any, _ int) string {
	return getFunctionName(i)
})

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
