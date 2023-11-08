package transcode

import (
	"fmt"
	"github.com/bcc-code/bccm-flows/paths"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/bcc-code/bccm-flows/common"
	"github.com/bcc-code/bccm-flows/services/ffmpeg"
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
	return fmt.Sprintf("[%s]aresample=48000,pan=1c|c0=c0[%s]", input, output)
}

func createSplitFilter(input string, count int) (string, []string) {
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
	type PlayoutLanguageState struct {
		Code       string
		FilePath   string
		CopyFrom   string
		InputIndex int
		Stereo     bool
	}

	params := []string{
		"-progress", "pipe:1",
		"-hide_banner",
	}

	// Inputs
	ffmpegInputCount := 0
	addInput := func(path paths.Path) {
		params = append(params, "-i", path.LocalPath())
		ffmpegInputCount++
	}
	addInput(input.VideoFilePath)

	_, fallbackExists := input.AudioFilePaths[input.FallbackLanguage]
	if !fallbackExists {
		return nil, fmt.Errorf("fallback audio file not found, fallbackLanguage is: %s", input.FallbackLanguage)
	}

	audioLanguages := [12]*PlayoutLanguageState{}
	for i, lang := range playoutLanguages {
		filePath, hasFile := input.AudioFilePaths[lang]

		inputIndex := -1
		copyFrom := ""
		if hasFile {
			inputIndex = ffmpegInputCount
			addInput(filePath)
		} else {
			copyFrom = input.FallbackLanguage
		}

		audioLanguages[i] = &PlayoutLanguageState{
			Code:       lang,
			FilePath:   filePath.LocalPath(),
			CopyFrom:   copyFrom,
			InputIndex: inputIndex,
			Stereo:     i < 4,
		}
	}

	leftStreams := map[string][]string{}
	useLeftStream := func(lang string) string {
		stream := leftStreams[lang][0]
		leftStreams[lang] = leftStreams[lang][1:]
		return stream
	}
	rightStreams := map[string][]string{}
	useRightStream := func(lang string) string {
		stream := rightStreams[lang][0]
		rightStreams[lang] = rightStreams[lang][1:]
		return stream
	}

	var filterParts []string
	for _, lang := range audioLanguages {
		if lang.InputIndex == -1 {
			continue
		}
		input := fmt.Sprintf("%d:a", lang.InputIndex)
		outputL := fmt.Sprintf("%s_l", lang.Code)
		outputR := fmt.Sprintf("%s_r", lang.Code)

		leftStreamsNeeded := 1
		rightStreamsNeeded := 0

		if lang.Stereo {
			filterParts = append(filterParts, createStereoFilter(input, outputL, outputR))
			rightStreamsNeeded++
		} else {
			filterParts = append(filterParts, createMonoFilter(input, outputL))
		}
		for _, m := range audioLanguages {
			if m.CopyFrom == lang.Code {
				leftStreamsNeeded++
				if m.Stereo {
					rightStreamsNeeded++
				}
			}
		}

		if leftStreamsNeeded == 1 {
			leftStreams[lang.Code] = []string{outputL}
		} else if leftStreamsNeeded > 1 {
			filter, labels := createSplitFilter(outputL, leftStreamsNeeded)
			filterParts = append(filterParts, filter)
			leftStreams[lang.Code] = labels
		}

		if rightStreamsNeeded == 1 {
			rightStreams[lang.Code] = []string{outputR}
		} else if rightStreamsNeeded > 1 {
			filter, labels := createSplitFilter(outputR, rightStreamsNeeded)
			filterParts = append(filterParts, filter)
			rightStreams[lang.Code] = labels
		}
	}

	params = append(params, "-filter_complex", strings.Join(filterParts, ";"))

	// Video must be first in the map
	params = append(params, "-map", "0:v")

	for _, f := range audioLanguages {
		lang := f.Code
		if f.CopyFrom != "" {
			lang = f.CopyFrom
		}
		label := useLeftStream(lang)
		params = append(params, "-map", fmt.Sprintf("[%s]", label))
		if f.Stereo {
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
	base := filepath.Base(input.VideoFilePath.LocalPath())
	fileNameWithoutExtension := base[:len(base)-len(filepath.Ext(base))]
	outputFilePath := filepath.Join(input.OutputDir.LocalPath(), fileNameWithoutExtension+".mxf")

	params, err := generateFFmpegParamsForPlayoutMux(input, outputFilePath)
	if err != nil {
		return nil, err
	}

	info, err := ffmpeg.GetStreamInfo(input.VideoFilePath.LocalPath())
	if err != nil {
		return nil, err
	}
	_, err = ffmpeg.Do(params, info, progressCallback)
	if err != nil {
		log.Default().Println("mux failed", err)
		return nil, fmt.Errorf("mux failed, %s", strings.Join(params, " "))
	}
	err = os.Chmod(outputFilePath, os.ModePerm)
	if err != nil {
		return nil, err
	}

	outputPath, err := paths.ParsePath(outputFilePath)
	if err != nil {
		return nil, err
	}

	return &common.PlayoutMuxResult{
		Path: outputPath,
	}, nil
}
