package activities

import (
	"reflect"
	"runtime"
	"strings"

	"github.com/bcc-code/bcc-media-flows/environment"

	"github.com/samber/lo"
)

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
	}
}

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
