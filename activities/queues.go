package activities

func GetAudioTranscodeActivities() []any {
	return []any{
		TranscodeToAudioAac,
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
