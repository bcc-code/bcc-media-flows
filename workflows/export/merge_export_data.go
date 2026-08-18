package export

import (
	"github.com/bcc-code/bcc-media-flows/activities"
	"github.com/bcc-code/bcc-media-flows/common"
	"github.com/bcc-code/bcc-media-flows/paths"
	"github.com/bcc-code/bcc-media-flows/services/vidispine"
	wfutils "github.com/bcc-code/bcc-media-flows/utils/workflows"
	"github.com/samber/lo"
	"go.temporal.io/sdk/workflow"
)

type MergeExportDataResult struct {
	Duration       float64
	VideoFile      *paths.Path
	AudioFiles     map[string]paths.Path
	SubtitleFiles  map[string]paths.Path
	JSONTranscript map[string]paths.Path
}

type MergeExportDataParams struct {
	ExportData                *vidispine.ExportData
	SubtitlesDir              paths.Path
	TempDir                   paths.Path
	MakeVideo                 bool
	MakeSubtitles             bool
	MakeAudio                 bool
	MakeTranscript            bool
	Languages                 []string
	OriginalLanguage          string
	ForceReplaceTranscription bool
}

func MergeExportData(ctx workflow.Context, params MergeExportDataParams) (*MergeExportDataResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting MergeExportData")
	data := params.ExportData

	dataMergeInputs := exportDataToMergeInputs(data, params.TempDir, params.SubtitlesDir)

	mergeInput := dataMergeInputs.MergeInput
	audioMergeInputs := dataMergeInputs.AudioMergeInputs
	subtitleMergeInputs := dataMergeInputs.SubtitleMergeInputs
	jsonTranscriptFile := dataMergeInputs.JSONTranscriptInput

	ctx = workflow.WithActivityOptions(ctx, wfutils.GetDefaultActivityOptions())

	var transcriptTask wfutils.Task[*activities.MergeTranscriptResult]
	if params.MakeTranscript && jsonTranscriptFile != nil {
		transcriptTask = wfutils.Execute(ctx, activities.Util.MergeTranscriptJSON, activities.MergeTranscriptJSONParams{
			MergeInput:      *jsonTranscriptFile,
			DestinationPath: params.TempDir,
		})
	}

	var audioTasks = map[string]wfutils.Task[*common.MergeResult]{}
	if params.MakeAudio {
		keys, err := wfutils.GetMapKeysSafely(ctx, audioMergeInputs)
		if err != nil {
			return nil, err
		}
		for _, lang := range keys {
			if len(params.Languages) != 0 && !lo.Contains(params.Languages, lang) {
				continue
			}
			mi := audioMergeInputs[lang]
			audioTasks[lang] = wfutils.Execute(ctx, activities.Audio.TranscodeMergeAudio, *mi)
		}
	}

	var subtitleTasks = map[string]wfutils.Task[*common.MergeResult]{}
	if params.MakeSubtitles {
		keys, err := wfutils.GetMapKeysSafely(ctx, subtitleMergeInputs)
		if err != nil {
			return nil, err
		}
		for _, lang := range keys {
			mi := subtitleMergeInputs[lang]
			subtitleTasks[lang] = wfutils.Execute(ctx, activities.Video.TranscodeMergeSubtitles, *mi)
		}

	}

	var videoFile *paths.Path
	if params.MakeVideo {
		result, err := wfutils.Execute(ctx, activities.Video.TranscodeMergeVideo, mergeInput).Result(ctx)
		if err != nil {
			return nil, err
		}
		videoFile = &result.Path
	}

	audioFiles, err := collectMergedPaths(ctx, audioTasks)
	if err != nil {
		return nil, err
	}

	subtitleFiles, err := collectMergedPaths(ctx, subtitleTasks)
	if err != nil {
		return nil, err
	}

	jsonTranscriptResult := map[string]paths.Path{}

	if params.MakeTranscript && transcriptTask.Future != nil {
		res, err := transcriptTask.Result(ctx)
		if err != nil {
			return nil, err
		}

		transcriptLang := "no"

		if data.OriginalLanguage != "" {
			transcriptLang = data.OriginalLanguage
		}

		if data.TranscribedLanguage != "" {
			transcriptLang = data.TranscribedLanguage
		}

		jsonTranscriptResult[transcriptLang] = res.Path
	}

	return &MergeExportDataResult{
		Duration:       mergeInput.Duration,
		VideoFile:      videoFile,
		AudioFiles:     audioFiles,
		SubtitleFiles:  subtitleFiles,
		JSONTranscript: jsonTranscriptResult,
	}, nil
}

type MergeInput struct {
	MergeInput          common.MergeInput
	AudioMergeInputs    map[string]*common.MergeInput
	SubtitleMergeInputs map[string]*common.MergeInput
	JSONTranscriptInput *common.MergeInput
}

func exportDataToMergeInputs(data *vidispine.ExportData, tempDir, subtitlesDir paths.Path) MergeInput {
	var JSONTranscriptInput *common.MergeInput

	mergeInput := common.MergeInput{
		Title:     data.SafeTitle,
		OutputDir: tempDir,
		WorkDir:   tempDir,
	}

	transcriptInput := &common.MergeInput{
		Title:     data.SafeTitle,
		OutputDir: tempDir,
		WorkDir:   tempDir,
		Items:     []common.MergeInputItem{},
	}

	audioMergeInputs := map[string]*common.MergeInput{}
	subtitleMergeInputs := map[string]*common.MergeInput{}

	for _, clip := range data.Clips {
		mergeInput.Duration += clip.OutSeconds - clip.InSeconds
		mergeInput.Items = append(mergeInput.Items, common.MergeInputItem{
			Path:  paths.MustParse(clip.VideoFile),
			Start: clip.InSeconds,
			End:   clip.OutSeconds,
		})

		if clip.JSONTranscriptFile != "" {
			transcriptInput.Duration += clip.OutSeconds - clip.InSeconds
			transcriptInput.Items = append(transcriptInput.Items, common.MergeInputItem{
				Path:  paths.MustParse(clip.JSONTranscriptFile),
				Start: clip.InSeconds,
				End:   clip.OutSeconds,
			})
		}

		// Sorted: this runs in workflow code, where map order must not leak.
		for _, lan := range wfutils.SortedKeys(clip.AudioFiles) {
			af := clip.AudioFiles[lan]
			if _, ok := audioMergeInputs[lan]; !ok {
				audioMergeInputs[lan] = &common.MergeInput{
					Title:     data.SafeTitle + "-" + lan,
					OutputDir: tempDir,
					WorkDir:   tempDir,
				}
			}

			audioMergeInputs[lan].Duration += clip.OutSeconds - clip.InSeconds
			audioMergeInputs[lan].Items = append(audioMergeInputs[lan].Items, common.MergeInputItem{
				Path:    paths.MustParse(af.File),
				Start:   clip.InSeconds,
				End:     clip.OutSeconds,
				Streams: af.Streams,
			})
		}

		for _, lan := range wfutils.SortedKeys(clip.SubtitleFiles) {
			sf := clip.SubtitleFiles[lan]
			if _, ok := subtitleMergeInputs[lan]; !ok {
				subtitleMergeInputs[lan] = &common.MergeInput{
					Title:     data.SafeTitle + "-" + lan,
					OutputDir: subtitlesDir,
					WorkDir:   tempDir,
				}
			}

			subtitleMergeInputs[lan].Duration += clip.OutSeconds - clip.InSeconds
			subtitleMergeInputs[lan].Items = append(subtitleMergeInputs[lan].Items, common.MergeInputItem{
				Path:  paths.MustParse(sf),
				Start: clip.InSeconds,
				End:   clip.OutSeconds,
			})
		}
	}

	if transcriptInput.Duration > 0 {
		JSONTranscriptInput = transcriptInput
	}

	return MergeInput{
		MergeInput:          mergeInput,
		AudioMergeInputs:    audioMergeInputs,
		SubtitleMergeInputs: subtitleMergeInputs,
		JSONTranscriptInput: JSONTranscriptInput,
	}
}

// collectMergedPaths waits for one merge per language and keeps where each landed.
func collectMergedPaths(ctx workflow.Context, tasks map[string]wfutils.Task[*common.MergeResult]) (map[string]paths.Path, error) {
	langs, err := wfutils.GetMapKeysSafely(ctx, tasks)
	if err != nil {
		return nil, err
	}

	files := map[string]paths.Path{}
	for _, lang := range langs {
		result, err := tasks[lang].Result(ctx)
		if err != nil {
			return nil, err
		}

		files[lang] = result.Path
	}

	return files, nil
}
