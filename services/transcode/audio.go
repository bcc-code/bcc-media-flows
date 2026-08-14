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
	outputFilePath := bitrateSuffixedOutput(input, "aac")

	_, err := ffmpeg.Run(ffmpeg.Job{
		Input:  input.Path.Local(),
		Output: outputFilePath,
		Args: []string{
			"-c:a", "libfdk_aac",
			"-b:a", input.Bitrate,
		},
	}, cb)
	if err != nil {
		return nil, err
	}

	return audioResult(outputFilePath, input.Bitrate, "aac")
}

// PrepareForTranscription prepares the audio file for transcription by converting it to a mono wav file
func PrepareForTranscription(input common.AudioInput, cb ffmpeg.ProgressCallback) (*common.AudioResult, error) {
	outputFilePath := bitrateSuffixedOutput(input, "wav")

	_, err := ffmpeg.Run(ffmpeg.Job{
		Input:  input.Path.Local(),
		Output: outputFilePath,
		Args: []string{
			"-map", "0:a:0",
			"-ac", "1",
		},
	}, cb)
	if err != nil {
		return nil, err
	}

	return audioResult(outputFilePath, input.Bitrate, "wav")
}

func AudioWav(input common.WavAudioInput, cb ffmpeg.ProgressCallback) (*common.AudioResult, error) {
	outputFilePath := input.DestinationPath.Append(input.Path.BaseNoExt() + ".wav").Local()

	args := []string{"-codec:a", "pcm_s24le"}

	if input.Timecode != "" {
		tcSamples, err := utils.TCToSamples(input.Timecode, 25, 48000)
		if err != nil {
			return nil, err
		}
		args = append(args,
			"-metadata", fmt.Sprintf("time_reference=%d", tcSamples),
			"-write_bext", "1",
		)
	}

	_, err := ffmpeg.Run(ffmpeg.Job{
		Input:  input.Path.Local(),
		Output: outputFilePath,
		Args:   args,
	}, cb)
	if err != nil {
		return nil, err
	}

	return audioResult(outputFilePath, "", "wav")
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
	params := []string{"-c:a", "libmp3lame"}

	if input.ForceCBR {
		params = append(params, "-b:a", input.Bitrate)
	} else {
		params = append(params, "-q:a", fmt.Sprint(getQfactorFromBitrate(input.Bitrate)))
	}

	outputFilePath := bitrateSuffixedOutput(input, "mp3")

	_, err := ffmpeg.Run(ffmpeg.Job{
		Input:  input.Path.Local(),
		Output: outputFilePath,
		Args:   params,
	}, cb)
	if err != nil {
		return nil, err
	}

	return audioResult(outputFilePath, input.Bitrate, "mp3")
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
	params := []string{"-ss", fmt.Sprintf("%f", start)}

	if end != 0 {
		params = append(params,
			"-to", fmt.Sprintf("%f", end))
	}

	params = append(params,
		"-map", "0",
		"-c", "copy")

	_, err := ffmpeg.Run(ffmpeg.Job{
		Input:  inFile.Local(),
		Output: outFile.Local(),
		Args:   params,
		Info:   &ffmpeg.StreamInfo{},
	}, cb)
	return err
}

func Convert51to4Mono(inFile, outFile paths.Path, cb ffmpeg.ProgressCallback) error {
	params := []string{
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
	}

	_, err := ffmpeg.Run(ffmpeg.Job{
		Input:  inFile.Local(),
		Output: outFile.Local(),
		Args:   params,
		Info:   &ffmpeg.StreamInfo{},
	}, cb)
	return err
}

// bitrateSuffixedOutput is the "<destination>/<input base without extension>-<bitrate>.<ext>"
// naming the audio encoders share.
func bitrateSuffixedOutput(input common.AudioInput, ext string) string {
	name := fmt.Sprintf("%s-%s.%s", input.Path.BaseNoExt(), input.Bitrate, ext)
	return input.DestinationPath.Append(name).Local()
}

// audioResult stats the finished file and describes it.
func audioResult(outputFilePath, bitrate, format string) (*common.AudioResult, error) {
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
		Bitrate:    bitrate,
		Format:     format,
		FileSize:   fileInfo.Size(),
	}, nil
}
