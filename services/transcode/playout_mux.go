package transcode

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/bcc-code/bccm-flows/common"
	"github.com/bcc-code/bccm-flows/services/ffmpeg"
	"github.com/samber/lo"
)

/***

# Playout mux

The playout system has these known requirements:
- Exactly 16 audio tracks representing 12 languages
- The audio tracks are all mono
- The first 8 audio tracks are stereo pairs (L and R) for the 4 first languages
- The next 8 audio tracks are mono for the 8 next languages

It's not known if this is a requirement, but we also:
- resample audio to 48khz (ffmpeg anyway cant use 44.1khz for pcm? got an error)
- reencode audio to 24bit pcm_s24le

## Example ffmpeg command
The following is a simpler version, to make it easier to understand the filter.

- Resample all audio to 48khz (aresample=48000)
- It creates 5 audio tracks.
  - Track 1 and 2 will be norwegian L and R (channelsplit=channel_layout=stereo)
  - Track 3 and 4 will be english L and R. (channelsplit=channel_layout=stereo)
  - Track 5 will be finnish L (pan=1c|c0=c0)
  - Track 6 will be norwegian L (the norwegian L is split into two streams: `asplit=2[nor_l_copy1][nor_l_copy2]`)
- Subtitles are not included.

```bash

ffmpeg \
  -i BERG_TS01_ISRAEL_VOD.mxf \
  -i BERG_TS01_ISRAEL_VOD-nor.wav \
  -i BERG_TS01_ISRAEL_VOD-eng.wav \
  -i BERG_TS01_ISRAEL_VOD-fin.wav \
  -filter_complex "[1:a]aresample=48000,channelsplit=channel_layout=stereo[nor_l][nor_r]; [nor_l]asplit=2[nor_l_copy1][nor_l_copy2]; [2:a]aresample=48000,channelsplit=channel_layout=stereo[eng_l][eng_r]; [3:a]aresample=48000,pan=1c|c0=c0[fin_l]" \
  -map "0:v" -map "[nor_l_copy1]" -map "[nor_r]" -map "[eng_l]" -map "[eng_r]" -map "[fin_l]" -map "[nor_l_copy2]" \
  -c:v copy -c:a pcm_s24le \
  -y transcoded/output.mxf

```

**/

var playoutLanguages = [12]string{
	"nor",
	"deu",
	"nld",
	"eng",
	"fra",
	"spa",
	"fin",
	"rus",
	"por",
	"ron",
	"tur",
	"pol",
}

func createStereoFilter(input string, leftOutput string, rightOutput string) string {
	return fmt.Sprintf("[%s]aresample=48000,channelsplit=channel_layout=stereo[%s][%s]", input, leftOutput, rightOutput)
}

func createMonoFilter(input string, output string) string {
	return fmt.Sprintf("[%s]aresample=48000[%s]", input, output)
}

func createSplitFilter(input string, splitCount int) string {
	filter := fmt.Sprintf("[%s]asplit=%d", input, splitCount)
	for i := 0; i < splitCount; i++ {
		filter += fmt.Sprintf("[%s_copy_%d]", input, i)
	}
	return filter
}

type inputFile struct {
	languageFile
	inputIndex int
}

type trackMap struct {
	file        inputFile
	stereo      bool
	copyFrom    string
	streamLabel string
}

func generateAudioSplitFilter(input string, count int) (string, []string) {
	filter := fmt.Sprintf("[%s]asplit=%d", input, count)
	labels := []string{}

	for i := 0; i < count; i++ {
		copyLabel := fmt.Sprintf("%s_copy_%d", input, i)
		filter += fmt.Sprintf("[%s]", copyLabel)
		labels = append(labels, copyLabel)
	}

	return filter, labels
}

func generateFFmpegParamsForPlayoutMux(input common.PlayoutMuxInput, outputPath string) ([]string, error) {
	params := []string{
		"-progress", "pipe:1",
		"-hide_banner",
	}
	ffmpegInputCount := 0
	addInput := func(path string) {
		params = append(params, "-i", path)
		ffmpegInputCount++
	}
	addInput(input.VideoFilePath)

	audioFiles := lo.Reduce(languageFilesForPaths(input.AudioFilePaths), func(agg []inputFile, item languageFile, index int) []inputFile {
		if item.Path == "" {
			return agg
		}
		return append(agg, inputFile{
			languageFile: item,
			inputIndex:   -1,
		})
	}, []inputFile{})

	fallbackLanguage, fallbackLanguageFound := lo.Find(audioFiles, func(f inputFile) bool {
		return f.Language == input.FallbackLanguage
	})
	if !fallbackLanguageFound {
		return nil, fmt.Errorf("fallback audio file not found, fallbackLanguage is: %s", input.FallbackLanguage)
	}

	trackMaps := [12]*trackMap{}
	for i, lang := range playoutLanguages {
		file, found := lo.Find(audioFiles, func(f inputFile) bool {
			return f.Language == lang
		})
		copyFrom := ""
		if !found {
			copyFrom = fallbackLanguage.Language
		}
		trackMaps[i] = &trackMap{
			file:     file,
			copyFrom: copyFrom,
			stereo:   i < 4,
		}
	}

	tracksWithInputFile := lo.Filter(trackMaps[:], func(f *trackMap, i int) bool {
		return f.file.Path != ""
	})

	for _, f := range tracksWithInputFile {
		f.file.inputIndex = ffmpegInputCount
		addInput(f.file.Path)
	}

	leftStreams := map[string][]string{}
	useLeftStream := func(lang string) string {
		copy := leftStreams[lang][0]
		leftStreams[lang] = leftStreams[lang][1:]
		return copy
	}
	rightStreams := map[string][]string{}
	useRightStream := func(lang string) string {
		copy := rightStreams[lang][0]
		rightStreams[lang] = rightStreams[lang][1:]
		return copy
	}

	filterParts := []string{}
	for _, track := range tracksWithInputFile {
		input := fmt.Sprintf("%d:a", track.file.inputIndex)
		outputL := fmt.Sprintf("%s_l", track.file.Language)
		outputR := fmt.Sprintf("%s_r", track.file.Language)

		leftStreamsNeeded := 1
		rightStreamsNeeded := 0

		if track.stereo {
			filterParts = append(filterParts, createStereoFilter(input, outputL, outputR))
			rightStreamsNeeded++
		} else {
			filterParts = append(filterParts, createMonoFilter(input, outputL))
		}
		for _, m := range trackMaps {
			if m.copyFrom == track.file.Language {
				leftStreamsNeeded++
				if m.stereo {
					rightStreamsNeeded++
				}
			}
		}

		if leftStreamsNeeded == 1 {
			leftStreams[track.file.Language] = []string{outputL}
		} else if leftStreamsNeeded > 1 {
			filter, labels := generateAudioSplitFilter(outputL, leftStreamsNeeded)
			filterParts = append(filterParts, filter)
			leftStreams[track.file.Language] = labels
		}

		if rightStreamsNeeded == 1 {
			rightStreams[track.file.Language] = []string{outputR}
		} else if rightStreamsNeeded > 1 {
			filter, labels := generateAudioSplitFilter(outputR, rightStreamsNeeded)
			filterParts = append(filterParts, filter)
			rightStreams[track.file.Language] = labels
		}

	}

	params = append(params, "-filter_complex", strings.Join(filterParts, ";"))

	// Video must be first in the map
	params = append(params, "-map", "0:v")

	for _, f := range trackMaps {
		lang := f.file.Language
		if f.copyFrom != "" {
			lang = f.copyFrom
		}
		label := useLeftStream(lang)
		params = append(params, "-map", fmt.Sprintf("[%s]", label))
		if f.stereo {
			label = useRightStream(lang)
			params = append(params, "-map", fmt.Sprintf("[%s]", label))
		}
	}
	params = append(params,
		"-c:v", "copy",
		"-c:a", "pcm_s24le",
		"-y", outputPath,
	)
	return params, nil
}

func PlayoutMux(input common.PlayoutMuxInput, progressCallback ffmpeg.ProgressCallback) (*common.PlayoutMuxResult, error) {
	outputPath := filepath.Join(input.DestinationPath, input.FileName+".mxf")

	params, err := generateFFmpegParamsForPlayoutMux(input, outputPath)
	if err != nil {
		return nil, err
	}

	info, err := ffmpeg.GetStreamInfo(input.VideoFilePath)
	if err != nil {
		return nil, err
	}
	print(strings.Join(params, "\n"))
	_, err = ffmpeg.Do(params, info, progressCallback)
	if err != nil {
		log.Default().Println("mux failed", err)
		return nil, fmt.Errorf("mux failed, %s", strings.Join(params, " "))
	}
	err = os.Chmod(outputPath, os.ModePerm)
	if err != nil {
		return nil, err
	}
	return &common.PlayoutMuxResult{
		Path: outputPath,
	}, nil
}
