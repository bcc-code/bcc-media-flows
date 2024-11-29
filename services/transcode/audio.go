package transcode

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/bcc-code/bcc-media-flows/utils"

	"github.com/bcc-code/bcc-media-flows/paths"

	"github.com/bcc-code/bcc-media-flows/common"
	"github.com/bcc-code/bcc-media-flows/services/ffmpeg"
)

type SilencePeriod struct {
	Start float64 `json:"start"`
	End   float64 `json:"end"`
}

func audioGetSilencePeriodsForRange(path paths.Path, threshold float64, from float64, length float64, stream int) ([]SilencePeriod, error) {
	params := []string{
		"-loglevel", "info",
		"-hide_banner",
		"-i", path.Local(),
		"-map", fmt.Sprintf("0:%d", stream),
		"-ss", fmt.Sprintf("%f", from),
		"-t", fmt.Sprintf("%f", length),
		"-af", fmt.Sprintf("silencedetect=noise=-70dB:d=%f", threshold),
		"-f", "null",
		"-",
	}

	fmt.Println(strings.Join(params, " "))

	cmd := exec.Command("ffmpeg", params...)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	// Run the command
	err := cmd.Run()
	if err != nil {
		fmt.Println(fmt.Sprint(err) + ": " + stderr.String())
		return nil, err
	}

	result := stderr.String()

	var silencePeriods []SilencePeriod
	r := regexp.MustCompile(`silence_(start|end): ([0-9.]+)`)

	var start float64
	for _, line := range strings.Split(result, "\n") {
		matches := r.FindStringSubmatch(line)
		if len(matches) == 3 {
			if matches[1] == "start" {
				start, _ = strconv.ParseFloat(matches[2], 64)
			} else if matches[1] == "end" {
				end, _ := strconv.ParseFloat(matches[2], 64)
				silencePeriods = append(silencePeriods, SilencePeriod{Start: start, End: end})
			}
		}
	}

	return silencePeriods, nil
}

func AudioStreamIsSilent(path paths.Path, stream int, from float64, to float64) (bool, error) {
	length := 30.0
	for i := from; i < to; i += length - i {
		silencePeriods, err := audioGetSilencePeriodsForRange(path, 5, i, length, stream)
		if err != nil {
			return false, err
		}

		var dur int
		for _, p := range silencePeriods {
			dur += int(p.End - p.Start - i)
		}

		if dur < int(length) && int(i)+dur < int(to) {
			return false, nil
		}

		length *= 2
	}

	return true, nil
}

func AudioIsSilent(path paths.Path) (bool, error) {
	info, err := ffmpeg.GetStreamInfo(path.Local())
	if err != nil {
		return false, err
	}

	return AudioStreamIsSilent(path, 0, 0, info.TotalSeconds)
}

func AudioAac(input common.AudioInput, cb ffmpeg.ProgressCallback) (*common.AudioResult, error) {
	params := []string{
		"-progress", "pipe:1",
		"-hide_banner",
		"-i", input.Path.Local(),
		"-c:a", "libfdk_aac",
		"-b:a", input.Bitrate,
	}

	outputFilePath := filepath.Join(input.DestinationPath.Local(), input.Path.Base())
	outputFilePath = fmt.Sprintf("%s-%s.aac", outputFilePath[:len(outputFilePath)-len(filepath.Ext(outputFilePath))], input.Bitrate)

	params = append(params, "-y", outputFilePath)

	info, err := ffmpeg.GetStreamInfo(input.Path.Local())
	if err != nil {
		return nil, err
	}

	_, err = ffmpeg.Do(params, info, cb)
	if err != nil {
		return nil, err
	}

	err = os.Chmod(outputFilePath, os.ModePerm)
	if err != nil {
		return nil, err
	}

	outputPath, err := paths.Parse(outputFilePath)
	if err != nil {
		return nil, err
	}

	fileInfo, err := os.Stat(outputFilePath)
	if err != nil {
		return nil, err
	}

	return &common.AudioResult{
		OutputPath: outputPath,
		Bitrate:    input.Bitrate,
		Format:     "aac",
		FileSize:   fileInfo.Size(),
	}, nil
}

// PrepareForTranscriptoion prepares the audio file for transcription by converting it to a mono wav file
func PrepareForTranscriptoion(input common.AudioInput, cb ffmpeg.ProgressCallback) (*common.AudioResult, error) {
	outputFilePath := filepath.Join(input.DestinationPath.Local(), input.Path.Base())
	outputFilePath = fmt.Sprintf("%s-%s.wav", outputFilePath[:len(outputFilePath)-len(filepath.Ext(outputFilePath))], input.Bitrate)

	params := []string{
		"-progress", "pipe:1",
		"-hide_banner",
		"-y",
		"-i", input.Path.Local(),
		"-map", "0:a:0",
		"-ac", "1",
		outputFilePath,
	}

	info, err := ffmpeg.GetStreamInfo(input.Path.Local())
	if err != nil {
		return nil, err
	}

	_, err = ffmpeg.Do(params, info, cb)
	if err != nil {
		return nil, err
	}

	err = os.Chmod(outputFilePath, os.ModePerm)
	if err != nil {
		return nil, err
	}

	outputPath, err := paths.Parse(outputFilePath)
	if err != nil {
		return nil, err
	}

	fileInfo, err := os.Stat(outputFilePath)
	if err != nil {
		return nil, err
	}

	return &common.AudioResult{
		OutputPath: outputPath,
		Bitrate:    input.Bitrate,
		Format:     "wav",
		FileSize:   fileInfo.Size(),
	}, nil
}

func AudioWav(input common.WavAudioInput, cb ffmpeg.ProgressCallback) (*common.AudioResult, error) {
	outputFilePath := input.DestinationPath.Append(input.Path.BaseNoExt() + ".wav").Local()

	params := []string{
		"-progress", "pipe:1",
		"-hide_banner",
		"-codec:a", "pcm_s24le",
		"-i", input.Path.Local(),
	}

	if input.Timecode != "" {
		tcSamples, err := utils.TCToSamples(input.Timecode, 25, 48000)
		if err != nil {
			return nil, err
		}
		params = append(params, "-metadata", fmt.Sprintf("time_reference=%d", tcSamples))
		params = append(params, "-write_bext", "1")
	}

	params = append(params, "-y", outputFilePath)

	info, err := ffmpeg.GetStreamInfo(input.Path.Local())
	if err != nil {
		return nil, err
	}

	_, err = ffmpeg.Do(params, info, cb)
	if err != nil {
		return nil, err
	}

	err = os.Chmod(outputFilePath, os.ModePerm)
	if err != nil {
		return nil, err
	}

	outputPath, err := paths.Parse(outputFilePath)
	if err != nil {
		return nil, err
	}

	fileInfo, err := os.Stat(outputFilePath)
	if err != nil {
		return nil, err
	}

	return &common.AudioResult{
		OutputPath: outputPath,
		Format:     "wav",
		FileSize:   fileInfo.Size(),
	}, nil
}

func getQfactorFromBitrate(input string) int {

	bitrate, _ := strconv.ParseInt(strings.ReplaceAll(input, "k", ""), 10, 64)

	switch {
	case bitrate >= 190:
		return 1
	case bitrate >= 170:
		return 2
	case bitrate >= 150:
		return 3
	case bitrate >= 140:
		return 4
	case bitrate >= 120:
		return 5
	case bitrate >= 100:
		return 6
	case bitrate >= 80:
		return 7
	case bitrate >= 70:
		return 8
	case bitrate >= 45:
		return 9
	default:
		return 1
	}
}

func AudioMP3(input common.AudioInput, cb ffmpeg.ProgressCallback) (*common.AudioResult, error) {
	params := []string{
		"-progress", "pipe:1",
		"-hide_banner",
		"-i", input.Path.Local(),
		"-c:a", "libmp3lame",
	}

	if input.ForceCBR {
		params = append(params, "-b:a", input.Bitrate)
	} else {
		params = append(params, "-q:a", fmt.Sprint(getQfactorFromBitrate(input.Bitrate)))
	}

	outputFilePath := filepath.Join(input.DestinationPath.Local(), input.Path.Base())
	outputFilePath = fmt.Sprintf("%s-%s.mp3", outputFilePath[:len(outputFilePath)-len(filepath.Ext(outputFilePath))], input.Bitrate)

	params = append(params, "-y", outputFilePath)

	info, err := ffmpeg.GetStreamInfo(input.Path.Local())
	if err != nil {
		return nil, err
	}

	_, err = ffmpeg.Do(params, info, cb)
	if err != nil {
		return nil, err
	}

	err = os.Chmod(outputFilePath, os.ModePerm)
	if err != nil {
		return nil, err
	}

	outputPath, err := paths.Parse(outputFilePath)
	if err != nil {
		return nil, err
	}

	fileInfo, err := os.Stat(outputFilePath)
	if err != nil {
		return nil, err
	}

	return &common.AudioResult{
		OutputPath: outputPath,
		Bitrate:    input.Bitrate,
		Format:     "mp3",
		FileSize:   fileInfo.Size(),
	}, nil
}

func SplitAudioChannels(filePath, outputDir paths.Path, cb ffmpeg.ProgressCallback) (paths.Files, error) {
	info, err := ffmpeg.ProbeFile(filePath.Local())
	if err != nil {
		return nil, err
	}

	params := []string{
		"-i", filePath.Local(),
	}

	var filter string

	var channels int
	for index, stream := range info.Streams {
		if stream.CodecType != "audio" {
			continue
		}
		for i := 0; i < stream.Channels; i++ {
			filter += fmt.Sprintf("[%d:a]pan=mono|c0=c%d[a%d];", index, i, channels)
			channels++
		}
	}

	var files paths.Files

	params = append(params, "-filter_complex", filter)

	for i := 0; i < channels; i++ {
		base := filePath.Base()
		fileName := fmt.Sprintf("%s-%d.wav", base[:len(base)-len(filepath.Ext(base))], i)
		file := outputDir.Append(fileName)
		files = append(files, file)
		params = append(params,
			"-map", fmt.Sprintf("[a%d]", i),
			file.Local(),
		)
	}

	_, err = ffmpeg.Do(params, ffmpeg.StreamInfo{}, cb)
	if err != nil {
		return nil, err
	}
	return files, nil
}

func ExtractAudioChannels(filePath paths.Path, output map[int]paths.Path, cb ffmpeg.ProgressCallback) (map[int]paths.Path, error) {
	if len(output) == 0 {
		return nil, fmt.Errorf("no output channels specified")
	}

	params := []string{
		"-progress", "pipe:1",
		"-hide_banner",
		"-i", filePath.Local(),
	}

	out := make(map[int]paths.Path)
	for channel, file := range output {
		params = append(params, "-map", fmt.Sprintf("0:%d", channel), "-c", "copy", file.Local())
	}

	_, err := ffmpeg.Do(params, ffmpeg.StreamInfo{}, cb)
	if err != nil {
		return nil, err
	}

	return out, nil
}

func GenerateToneFile(frequency int, duration float64, sampleRate int, timecode string, filePath paths.Path) error {
	samples, err := utils.TCToSamples(timecode, 25, sampleRate)
	if err != nil {
		return err
	}

	params := []string{
		"-f", "lavfi",
		"-i", fmt.Sprintf("sine=frequency=%d:sample_rate=%d:duration=%f", frequency, sampleRate, duration),
		"-codec:a", "pcm_s24le",
		"-metadata", fmt.Sprintf("time_reference=%d", samples),
		"-write_bext", "1",
		filePath.Local(),
	}

	_, err = ffmpeg.Do(params, ffmpeg.StreamInfo{}, nil)
	return err
}

func TrimFile(inFile, outFile paths.Path, start, end float64, cb ffmpeg.ProgressCallback) error {
	params := []string{
		"-progress", "pipe:1",
		"-hide_banner",
		"-y",
		"-i", inFile.Local(),
		"-ss", fmt.Sprintf("%f", start),
	}

	if end != 0 {
		params = append(params,
			"-to", fmt.Sprintf("%f", end))
	}

	params = append(params,
		"-c", "copy",
		outFile.Local())

	_, err := ffmpeg.Do(params, ffmpeg.StreamInfo{}, cb)
	return err
}

func Convert51to4Mono(inFile, outFile paths.Path, cb ffmpeg.ProgressCallback) error {
	params := []string{
		"-progress", "pipe:1",
		"-hide_banner",
		"-y",
		"-i", inFile.Local(),
		"-map", "0:v",
		"-c:v", "copy", // Copy video unchanged
		"-filter_complex", // Process audio
		"[0:a:0]channelsplit=channel_layout=5.1[FL][FR][FC][LFE][BL][BR];" + // Split the 5.1 stream into 6 mono streams
			"[LFE]anullsink;" + // Discard the LFE channel
			"[FC]anullsink;" + // Discard the FC channel
			"[FL]aformat=channel_layouts=mono[FL2];" + // Convert the channels to mono layout. Otherwise ffmpeg will complain about the channel layout
			"[FR]aformat=channel_layouts=mono[FR2];" +
			"[BL]aformat=channel_layouts=mono[BL2];" +
			"[BR]aformat=channel_layouts=mono[BR2];",
		"-map", "[FL2]", // Map the mono streams to the output
		"-map", "[FR2]",
		"-map", "[BL2]",
		"-map", "[BR2]",
		"-c:a", "pcm_s24le", // We can not use -c copy here, because the channel layout is changed, but this should be the default codec in any case
		outFile.Local(),
	}

	_, err := ffmpeg.Do(params, ffmpeg.StreamInfo{}, cb)
	return err
}
