package workflows

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	bccmflows "github.com/bcc-code/bccm-flows"
	"github.com/bcc-code/bccm-flows/activities"
	avidispine "github.com/bcc-code/bccm-flows/activities/vidispine"
	"github.com/bcc-code/bccm-flows/common"
	"github.com/bcc-code/bccm-flows/common/ingest"
	"github.com/bcc-code/bccm-flows/common/smil"
	"github.com/bcc-code/bccm-flows/services/vidispine"
	"github.com/bcc-code/bccm-flows/utils"
	"github.com/samber/lo"
	"go.temporal.io/sdk/workflow"
	"path/filepath"
	"strings"
)

type AssetExportParams struct {
	VXID string
}

type AssetExportResult struct {
	Duration string `json:"duration"`
	ID       string `json:"id"`
	SmilFile string `json:"smil_file"`
	Title    string `json:"title"`
}

const (
	r1080p = "1920x1080"
	r720p  = "1280x720"
	r540p  = "960x540"
	r360p  = "640x360"
	r270p  = "480x270"
	r180p  = "320x180"
)

func formatSecondsToTimestamp(seconds float64) string {
	hours := int(seconds / 3600)
	seconds -= float64(hours * 3600)

	minutes := int(seconds / 60)
	seconds -= float64(minutes * 60)

	secondsInt := int(seconds)
	frames := int((seconds - float64(secondsInt)) * 100)

	return fmt.Sprintf("%02d:%02d:%02d:%02d", hours, minutes, secondsInt, frames)
}

func exportDataToMergeInputs(data *vidispine.ExportData, tempFolder, subtitlesFolder string) (
	mergeInput common.MergeInput,
	audioMergeInputs map[string]*common.MergeInput,
	subtitleMergeInputs map[string]*common.MergeInput,
) {
	mergeInput = common.MergeInput{
		Title:     data.Title,
		OutputDir: tempFolder,
		WorkDir:   tempFolder,
	}

	audioMergeInputs = map[string]*common.MergeInput{}
	subtitleMergeInputs = map[string]*common.MergeInput{}

	for _, clip := range data.Clips {
		mergeInput.Duration += clip.OutSeconds - clip.InSeconds
		mergeInput.Items = append(mergeInput.Items, common.MergeInputItem{
			Path:  clip.VideoFile,
			Start: clip.InSeconds,
			End:   clip.OutSeconds,
		})

		for lan, af := range clip.AudioFiles {
			if _, ok := audioMergeInputs[lan]; !ok {
				audioMergeInputs[lan] = &common.MergeInput{
					Title:     data.Title + "-" + lan,
					OutputDir: tempFolder,
					WorkDir:   tempFolder,
				}
			}

			audioMergeInputs[lan].Items = append(audioMergeInputs[lan].Items, common.MergeInputItem{
				Path:    af.File,
				Start:   clip.InSeconds,
				End:     clip.OutSeconds,
				Streams: af.Channels,
			})
		}

		for lan, sf := range clip.SubtitleFiles {
			if _, ok := subtitleMergeInputs[lan]; !ok {
				subtitleMergeInputs[lan] = &common.MergeInput{
					Title:     data.Title + "-" + lan,
					OutputDir: subtitlesFolder,
					WorkDir:   tempFolder,
				}
			}

			subtitleMergeInputs[lan].Items = append(subtitleMergeInputs[lan].Items, common.MergeInputItem{
				Path:  sf,
				Start: clip.InSeconds,
				End:   clip.OutSeconds,
			})
		}
	}

	return
}

func AssetExportVX(ctx workflow.Context, params AssetExportParams) (*AssetExportResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting AssetExport")

	options := GetDefaultActivityOptions()
	ctx = workflow.WithActivityOptions(ctx, options)

	var data *vidispine.ExportData

	err := workflow.ExecuteActivity(ctx, avidispine.GetExportDataActivity, avidispine.GetExportDataParams{
		VXID: params.VXID,
	}).Get(ctx, &data)
	if err != nil {
		return nil, err
	}

	logger.Info("Retrieved data from vidispine")

	outputFolder, err := getWorkflowOutputFolder(ctx)
	if err != nil {
		return nil, err
	}

	tempFolder, err := getWorkflowTempFolder(ctx)
	if err != nil {
		return nil, err
	}

	filesFolder := filepath.Join(outputFolder, "files")
	err = createFolder(ctx, filesFolder)
	if err != nil {
		return nil, err
	}

	streamsFolder := filepath.Join(outputFolder, "streams")
	err = createFolder(ctx, streamsFolder)
	if err != nil {
		return nil, err
	}

	subtitlesFolder := filepath.Join(outputFolder, "subtitles")
	err = createFolder(ctx, subtitlesFolder)
	if err != nil {
		return nil, err
	}

	//defer func() {
	//	_ = os.RemoveAll(tempFolder)
	//}()

	mergeInput, audioMergeInputs, subtitleMergeInputs := exportDataToMergeInputs(data, tempFolder, subtitlesFolder)

	ingestData := ingest.Data{
		Title:    data.Title,
		Id:       params.VXID,
		Duration: formatSecondsToTimestamp(mergeInput.Duration),
	}

	options.TaskQueue = utils.GetTranscodeQueue()
	ctx = workflow.WithActivityOptions(ctx, options)

	var videoFile string
	{
		var result common.MergeResult
		err = workflow.ExecuteActivity(ctx, activities.TranscodeMergeVideo, mergeInput).Get(ctx, &result)
		if err != nil {
			return nil, err
		}
		videoFile = result.Path
	}

	var audioFiles = map[string]string{}
	for lang, mi := range audioMergeInputs {
		var result common.MergeResult
		err = workflow.ExecuteActivity(ctx, activities.TranscodeMergeAudio, *mi).Get(ctx, &result)
		if err != nil {
			return nil, err
		}
		audioFiles[lang] = result.Path
	}

	var subtitleFiles = map[string]string{}
	for lang, mi := range subtitleMergeInputs {
		var result common.MergeResult
		err = workflow.ExecuteActivity(ctx, activities.TranscodeMergeSubtitles, *mi).Get(ctx, &result)
		if err != nil {
			return nil, err
		}
		subtitleFiles[lang] = result.Path
	}

	videoFiles := map[string]string{}
	{
		qualities := map[string]common.VideoInput{
			r1080p: {
				Path:            videoFile,
				FrameRate:       25,
				Width:           1920,
				Height:          1080,
				Bitrate:         "5M",
				DestinationPath: tempFolder,
			},
			r720p: {
				Path:            videoFile,
				FrameRate:       25,
				Width:           1280,
				Height:          720,
				Bitrate:         "3M",
				DestinationPath: tempFolder,
			},
			r540p: {
				Path:            videoFile,
				FrameRate:       25,
				Width:           960,
				Height:          540,
				Bitrate:         "1900k",
				DestinationPath: tempFolder,
			},
			r360p: {
				Path:            videoFile,
				FrameRate:       25,
				Width:           640,
				Height:          360,
				Bitrate:         "980k",
				DestinationPath: tempFolder,
			},
			r270p: {
				Path:            videoFile,
				FrameRate:       25,
				Width:           480,
				Height:          270,
				Bitrate:         "610k",
				DestinationPath: tempFolder,
			},
			r180p: {
				Path:            videoFile,
				FrameRate:       25,
				Width:           320,
				Height:          180,
				Bitrate:         "320k",
				DestinationPath: tempFolder,
			},
		}
		for key, input := range qualities {
			var result common.VideoResult
			err = workflow.ExecuteActivity(ctx, activities.TranscodeToVideoH264, input).Get(ctx, &result)
			if err != nil {
				return nil, err
			}
			videoFiles[key] = result.OutputPath
		}
	}

	var compressedAudioFiles = map[string]string{}
	for lang, path := range audioFiles {
		var result common.AudioResult
		err = workflow.ExecuteActivity(ctx, activities.TranscodeToAudioAac, common.AudioInput{
			Path:            path,
			Bitrate:         "128k",
			DestinationPath: tempFolder,
		}).Get(ctx, &result)
		if err != nil {
			return nil, err
		}
		compressedAudioFiles[lang] = result.OutputPath
	}

	languages := utils.LanguageKeysToOrderedLanguages(lo.Keys(compressedAudioFiles))

	for _, lang := range languages {
		for _, q := range []string{r1080p, r540p, r180p} {
			base := filepath.Base(videoFiles[q])
			key := lang.ISO6391
			fileName := base[:len(base)-len(filepath.Ext(base))] + "-" + key
			var result common.MuxResult
			err = workflow.ExecuteActivity(ctx, activities.TranscodeMux, common.MuxInput{
				FileName:          fileName,
				DestinationPath:   filesFolder,
				VideoFilePath:     videoFiles[q],
				AudioFilePaths:    map[string]string{key: compressedAudioFiles[key]},
				SubtitleFilePaths: subtitleFiles,
			}).Get(ctx, &result)
			if err != nil {
				return nil, err
			}
			ingestData.Files = append(ingestData.Files, ingest.File{
				Resolution:    q,
				AudioLanguage: lang.ISO6392TwoLetter,
				Mime:          "video/mp4",
				Path:          filepath.Join("files", filepath.Base(result.Path)),
			})
		}
	}

	var smilData smil.Smil
	smilData.XMLName.Local = "smil"
	smilData.XMLName.Space = "http://www.w3.org/2001/SMIL20/Language"
	smilData.Head.Meta.Name = "formats"
	smilData.Head.Meta.Content = "mp4"

	subtitleLanguages := utils.LanguageKeysToOrderedLanguages(lo.Keys(subtitleFiles))
	for _, language := range subtitleLanguages {
		path := subtitleFiles[language.ISO6391]
		smilData.Body.Switch.TextStreams = append(smilData.Body.Switch.TextStreams, smil.TextStream{
			Src:            filepath.Join("subtitles", filepath.Base(path)),
			SystemLanguage: language.ISO6391,
			SubtitleName:   language.LanguageNameSystem,
		})
	}

	for _, q := range []string{r180p, r270p, r360p, r540p, r720p, r1080p} {

		path := videoFiles[q]

		audioFilePaths := map[string]string{}
		var fileLanguages []bccmflows.Language
		// Add audio files to mux, but uniquely across qualities.
		for len(languages) > 0 && len(audioFilePaths) < 16 {
			key := languages[0].ISO6391
			fileLanguages = append(fileLanguages, languages[0])
			audioFilePaths[key] = compressedAudioFiles[key]
			languages = languages[1:]
		}

		var result common.MuxResult
		err = workflow.ExecuteActivity(ctx, activities.TranscodeMux, common.MuxInput{
			FileName:        filepath.Base(path),
			DestinationPath: streamsFolder,
			AudioFilePaths:  audioFilePaths,
			VideoFilePath:   path,
		}).Get(ctx, &result)
		if err != nil {
			return nil, err
		}

		smilData.Body.Switch.Videos = append(smilData.Body.Switch.Videos, smil.Video{
			Src:          filepath.Join("streams", filepath.Base(result.Path)),
			IncludeAudio: fmt.Sprintf("%t", len(fileLanguages) > 0),
			SystemLanguage: strings.Join(lo.Map(fileLanguages, func(i bccmflows.Language, _ int) string {
				return i.ISO6391
			}), ","),
			AudioName: strings.Join(lo.Map(fileLanguages, func(i bccmflows.Language, _ int) string {
				return i.LanguageNameSystem
			}), ","),
		})
	}

	xmlData, _ := xml.MarshalIndent(smilData, "", "\t")
	err = writeFile(ctx, filepath.Join(outputFolder, "smil.xml"), xmlData)
	if err != nil {

		return nil, err
	}

	ingestData.SmilFile = "smil.xml"

	marshalled, err := json.Marshal(ingestData)
	if err != nil {
		return nil, err
	}

	err = writeFile(ctx, filepath.Join(outputFolder, "ingest.json"), marshalled)
	if err != nil {
		return nil, err
	}
	err = deletePath(ctx, tempFolder)
	return nil, err
}
