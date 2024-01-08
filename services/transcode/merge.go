package transcode

import (
	"fmt"
	"github.com/bcc-code/bcc-media-flows/paths"
	"log"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/bcc-code/bcc-media-flows/common"
	"github.com/bcc-code/bcc-media-flows/services/ffmpeg"
	"github.com/bcc-code/bcc-media-flows/utils"
	"github.com/samber/lo"
)

func getFramerate(input common.MergeInput) (int, error) {
	longestItem := lo.MaxBy(input.Items, func(a common.MergeInputItem, b common.MergeInputItem) bool {
		return a.End-a.Start > b.End-b.Start
	})

	info, err := ffmpeg.GetStreamInfo(longestItem.Path.Local())
	if err != nil {
		return 0, err
	}

	var rate = 25
	if info.FrameRate > 40 {
		rate = 50
	}

	return rate, nil
}

// MergeVideo takes a list of video files and merges them into one file.
func MergeVideo(input common.MergeInput, progressCallback ffmpeg.ProgressCallback) (*common.MergeResult, error) {
	var params []string

	for _, i := range input.Items {
		params = append(params, "-i", i.Path.Local())
	}

	var filterComplex string

	for index, i := range input.Items {
		// Add the video stream and timestamps to the filter, with setpts to let the transcoder know to continue the timestamp from the previous file.
		filterComplex += fmt.Sprintf("[%d:v]trim=start=%f:end=%f,setpts=PTS-STARTPTS,yadif[v%d];", index, i.Start, i.End, index)
	}

	for index := range input.Items {

		filterComplex += fmt.Sprintf("[v%d]", index)
	}

	rate, err := getFramerate(input)
	if err != nil {
		return nil, err
	}

	// Concatenate the video streams.
	filterComplex += fmt.Sprintf("concat=n=%d:v=1:a=0[v]", len(input.Items))

	outputFilePath := filepath.Join(input.OutputDir.Local(), filepath.Clean(input.Title)+".mxf")

	params = append(params,
		"-progress", "pipe:1",
		"-hide_banner",
		"-strict", "unofficial",
		"-filter_complex", filterComplex,
		"-map", "[v]",
		"-c:v", "prores",
		"-profile:v", "3",
		"-vendor", "ap10",
		"-bits_per_mb", "8000",
		"-r", strconv.Itoa(rate),
		"-pix_fmt", "yuv422p10le",
		"-color_primaries", "bt709",
		"-color_trc", "bt709",
		"-colorspace", "bt709",
		"-y",
		outputFilePath,
	)

	_, err = ffmpeg.Do(params, ffmpeg.StreamInfo{
		TotalSeconds: input.Duration,
	}, progressCallback)
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

	return &common.MergeResult{
		Path: outputPath,
	}, nil
}

// mergeItemToStereoStream takes a merge input item and returns a string that can be used in a filter_complex to merge the audio streams.
func mergeItemToStereoStream(index int, tag string, item common.MergeInputItem) (string, error) {
	path := item.Path.Local()
	info, _ := ffmpeg.ProbeFile(path)

	if info == nil || len(info.Streams) == 0 {
		return fmt.Sprintf("anullsrc=channel_layout=stereo[%s]", tag), nil
	}

	var streams []ffmpeg.FFProbeStream

	for _, stream := range item.Streams {
		s, found := lo.Find(info.Streams, func(s ffmpeg.FFProbeStream) bool {
			return s.Index == stream
		})
		if found {
			streams = append(streams, s)
		}
	}

	if len(streams) == 0 {
		s, found := lo.Find(info.Streams, func(s ffmpeg.FFProbeStream) bool {
			return s.ChannelLayout == "stereo" && s.Channels == 2
		})
		if found {
			streams = append(streams, s)
		}
	}

	var streamString string
	channels := 0
	for _, stream := range streams {
		if stream.ChannelLayout == "stereo" && stream.Channels == 2 {
			return fmt.Sprintf("[%d:%d]aselect[%s]", index, stream.Index, tag), nil
		} else {
			streamString += fmt.Sprintf("[%d:%d]", index, stream.Index)
			channels++
		}
	}
	if channels == 0 {
		streamString += fmt.Sprintf("anullsrc=channel_layout=stereo[%s]", tag)
	} else if channels == 2 {
		streamString += fmt.Sprintf("amerge=inputs=2[%s]", tag)
	} else {
		streamString += fmt.Sprintf("amerge=inputs=%d[%s]", channels, tag)
	}

	return streamString, nil
}

// MergeAudio merges MXF audio files into one stereo file.
func MergeAudio(input common.MergeInput, progressCallback ffmpeg.ProgressCallback) (*common.MergeResult, error) {
	params := []string{
		"-progress", "pipe:1",
		"-hide_banner",
	}

	for _, i := range input.Items {
		params = append(params, "-i", i.Path.Local())
	}

	params = append(params,
		"-c:a", "pcm_s16le",
	)

	var filterComplex string

	for index, i := range input.Items {
		r, err := mergeItemToStereoStream(index, fmt.Sprintf("a%d", index), i)
		if err != nil {
			return nil, err
		}
		filterComplex += r + ";"
	}

	for index, i := range input.Items {
		filterComplex += fmt.Sprintf("[a%d]atrim=start=%f:end=%f,asetpts=PTS-STARTPTS[a%[1]d_trimmed];", index, i.Start, i.End)
	}
	for index := range input.Items {
		filterComplex += fmt.Sprintf("[a%d_trimmed]", index)
	}

	filterComplex += fmt.Sprintf("concat=n=%d:v=0:a=1 [a]", len(input.Items))

	outputFilePath := filepath.Join(input.OutputDir.Local(), filepath.Clean(input.Title)+".wav")

	params = append(params, "-filter_complex", filterComplex, "-map", "[a]", "-y", outputFilePath)

	log.Default().Println(strings.Join(params, " "))
	_, err := ffmpeg.Do(params, ffmpeg.StreamInfo{}, progressCallback)
	if err != nil {
		return nil, err
	}

	outputPath, err := paths.Parse(outputFilePath)
	if err != nil {
		return nil, err
	}

	return &common.MergeResult{
		Path: outputPath,
	}, err
}

func formatDuration(seconds float64) string {
	// Calculate hours, minutes, and whole seconds
	hours := int(seconds) / 3600
	minutes := int(seconds) / 60 % 60
	wholeSeconds := int(seconds) % 60

	// Calculate milliseconds
	milliseconds := int(math.Mod(seconds, 1) * 1000)

	// Return the formatted string
	return fmt.Sprintf("%02d:%02d:%02d,%03d", hours, minutes, wholeSeconds, milliseconds)
}

func MergeSubtitles(input common.MergeInput, progressCallback ffmpeg.ProgressCallback) (*common.MergeResult, error) {
	var files []string
	// for each file, extract the specified range and save the result to a file.

	startAt := 0.0
	for index, item := range input.Items {
		file := filepath.Join(input.WorkDir.Local(), fmt.Sprintf("%s-%d.srt", input.Title, index))
		fileOut := filepath.Join(input.WorkDir.Local(), fmt.Sprintf("%s-%d-out.srt", input.Title, index))
		path := item.Path.Local()

		cmd := exec.Command("ffmpeg", "-i", path, "-ss", fmt.Sprintf("%f", item.Start), "-to", fmt.Sprintf("%f", item.End), "-y", file)

		_, err := utils.ExecuteCmd(cmd, nil)
		if err != nil {
			return nil, err
		}
		fileInfo, err := os.Stat(file)
		if err != nil {
			return nil, err
		}
		if fileInfo.Size() == 0 {
			err = os.WriteFile(file, []byte(fmt.Sprintf("1\n%s --> %s\n", formatDuration(item.Start), formatDuration(item.End))), os.ModePerm)
			if err != nil {
				return nil, err
			}
		}

		cmd = exec.Command("ffmpeg", "-itsoffset", fmt.Sprintf("%f", startAt), "-i", file, "-y", fileOut)
		_, err = utils.ExecuteCmd(cmd, nil)
		if err != nil {
			return nil, err
		}
		startAt += item.End - item.Start

		files = append(files, fileOut)
	}

	// the files have to be present in a text file for ffmpeg to concatenate them.
	// #subtitles.txt
	// file /path/to/file/0.srt
	// file /path/to/file/1.srt
	var content string
	for _, f := range files {
		content += fmt.Sprintf("file '%s'\n", f)
	}

	subtitlesFile := filepath.Join(input.WorkDir.Local(), input.Title+"-subtitles.txt")

	err := os.WriteFile(subtitlesFile, []byte(content), os.ModePerm)
	if err != nil {
		return nil, err
	}

	concatStr := fmt.Sprintf("concat:%s", strings.Join(files, "|"))

	outputFilePath := filepath.Join(input.OutputDir.Local(), filepath.Clean(input.Title)+".srt")
	params := []string{
		"-progress", "pipe:1",
		"-hide_banner",
		"-i", concatStr,
		"-c", "copy",
		"-y",
		outputFilePath,
	}

	_, err = ffmpeg.Do(params, ffmpeg.StreamInfo{}, progressCallback)
	if err != nil {
		return nil, err
	}

	outputPath, err := paths.Parse(outputFilePath)
	if err != nil {
		return nil, err
	}

	return &common.MergeResult{
		Path: outputPath,
	}, err
}
