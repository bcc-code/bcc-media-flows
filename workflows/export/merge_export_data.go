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
	ExportData     *vidispine.ExportData
	SubtitlesDir   paths.Path
	TempDir        paths.Path
	MakeVideo      bool
	MakeSubtitles  bool
	MakeAudio      bool
	MakeTranscript bool
	Languages      []string
}

func MergeExportData(ctx workflow.Context, params MergeExportDataParams) (*MergeExportDataResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting MergeExportData")
	data := params.ExportData

	mergeInput, audioMergeInputs, subtitleMergeInputs, jsonTranscriptFile := exportDataToMergeInputs(data, params.TempDir, params.SubtitlesDir)

	ctx = workflow.WithActivityOptions(ctx, wfutils.GetDefaultActivityOptions())

	var transcriptTask workflow.Future
	if params.MakeTranscript && jsonTranscriptFile != nil {
		transcriptTask = wfutils.Execute(ctx, activities.Util.MergeTranscriptJSON, activities.MergeTranscriptJSONParams{
			MergeInput:      *jsonTranscriptFile,
			DestinationPath: params.TempDir,
		}).Future
	}

	var audioTasks = map[string]workflow.Future{}
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
			audioTasks[lang] = wfutils.Execute(ctx, activities.Audio.TranscodeMergeAudio, *mi).Future
		}
	}

	var subtitleTasks = map[string]workflow.Future{}
	if params.MakeSubtitles {
		keys, err := wfutils.GetMapKeysSafely(ctx, subtitleMergeInputs)
		if err != nil {
			return nil, err
		}
		for _, lang := range keys {
			mi := subtitleMergeInputs[lang]
			subtitleTasks[lang] = wfutils.Execute(ctx, activities.Video.TranscodeMergeSubtitles, *mi).Future
		}

	}

	var videoFile *paths.Path
	if params.MakeVideo {
		videoTask := wfutils.Execute(ctx, activities.Video.TranscodeMergeVideo, mergeInput)
		var result common.MergeResult
		err := videoTask.Get(ctx, &result)
		if err != nil {
			return nil, err
		}
		videoFile = &result.Path
	}

	var audioFiles = map[string]paths.Path{}
	{
		keys, err := wfutils.GetMapKeysSafely(ctx, audioTasks)
		if err != nil {
			return nil, err
		}
		for _, lang := range keys {
			task := audioTasks[lang]
			var result common.MergeResult
			err = task.Get(ctx, &result)
			if err != nil {
				return nil, err
			}
			audioFiles[lang] = result.Path
		}
	}

	var subtitleFiles = map[string]paths.Path{}
	{
		keys, err := wfutils.GetMapKeysSafely(ctx, subtitleTasks)
		if err != nil {
			return nil, err
		}
		for _, lang := range keys {
			task := subtitleTasks[lang]
			var result common.MergeResult
			err = task.Get(ctx, &result)
			if err != nil {
				return nil, err
			}
			subtitleFiles[lang] = result.Path
		}
	}

	jsonTranscriptResult := map[string]paths.Path{}

	if params.MakeTranscript && transcriptTask != nil {
		var res activities.MergeTranscriptResult
		err := transcriptTask.Get(ctx, &res)
		if err != nil {
			return nil, err
		}
		jsonTranscriptResult["no"] = res.Path
	}

	return &MergeExportDataResult{
		Duration:       mergeInput.Duration,
		VideoFile:      videoFile,
		AudioFiles:     audioFiles,
		SubtitleFiles:  subtitleFiles,
		JSONTranscript: jsonTranscriptResult,
	}, nil
}

func exportDataToMergeInputs(data *vidispine.ExportData, tempDir, subtitlesDir paths.Path) (
	mergeInput common.MergeInput,
	audioMergeInputs map[string]*common.MergeInput,
	subtitleMergeInputs map[string]*common.MergeInput,
	JSONTranscriptInput *common.MergeInput,
) {
	mergeInput = common.MergeInput{
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

	audioMergeInputs = map[string]*common.MergeInput{}
	subtitleMergeInputs = map[string]*common.MergeInput{}

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

		for lan, af := range clip.AudioFiles {
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

		for lan, sf := range clip.SubtitleFiles {
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

	return
}
