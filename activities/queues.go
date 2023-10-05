package activities

import (
	"reflect"
	"runtime"
	"strings"

	"github.com/bcc-code/bccm-flows/utils"
	"github.com/samber/lo"
)

func GetAudioTranscodeActivities() []any {
	return []any{
		TranscodeToAudioAac,
		TranscodeToAudioMP3,
		TranscodeToAudioWav,
		TranscodeMergeAudio,
		AnalyzeEBUR128Activity,
		AdjustAudioLevelActivity,
	}
}

func GetVideoTranscodeActivities() []any {
	return []any{
		TranscodePreview,
		TranscodeToProResActivity,
		TranscodeToH264Activity,
		TranscodeToXDCAMActivity,
		TranscodeMergeVideo,
		TranscodeMergeSubtitles,
		TranscodeToVideoH264,
		TranscodeMux,
		TranscodePlayoutMux,
		ExecuteFFmpeg,
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
		return utils.GetAudioQueue()
	}
	if lo.Contains(videoActivities, f) {
		return utils.GetTranscodeQueue()
	}
	return utils.GetWorkerQueue()
}
