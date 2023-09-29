package transcode

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/bcc-code/bccm-flows/common"
	"github.com/bcc-code/bccm-flows/services/ffmpeg"
	"github.com/bcc-code/bccm-flows/utils"
	"github.com/samber/lo"
)

// MergeVideo takes a list of video files and merges them into one file.
func MergeVideo(input common.MergeInput, progressCallback ffmpeg.ProgressCallback) (*common.MergeResult, error) {
	var params []string

	for _, i := range input.Items {
		params = append(params, "-i", utils.IsilonPathFix(i.Path))
	}

	var filterComplex string

	for index, i := range input.Items {
		// Add the video stream and timestamps to the filter, with setpts to let the transcoder know to continue the timestamp from the previous file.
		filterComplex += fmt.Sprintf("[%d:v] trim=start=%f:end=%f,setpts=PTS-STARTPTS,yadif[v%d];", index, i.Start, i.End, index)
	}

	filterComplex += " "
	for index := range input.Items {

		filterComplex += fmt.Sprintf("[v%d] ", index)
	}

	// Concatenate the video streams.
	filterComplex += fmt.Sprintf("concat=n=%d:v=1:a=0 [v]", len(input.Items))

	outputPath := filepath.Join(input.OutputDir, filepath.Clean(input.Title)+".mxf")

	params = append(params,
		"-progress", "pipe:1",
		"-hide_banner",
		"-filter_complex", filterComplex,
		"-map", "[v]",
		"-c:v", "prores",
		"-profile:v", "3",
		"-vendor", "ap10",
		"-pix_fmt", "yuv422p10le",
		"-color_primaries", "bt709",
		"-color_trc", "bt709",
		"-colorspace", "bt709",
		"-y",
		outputPath,
	)

	log.Default().Println(strings.Join(params, " "))

	_, err := ffmpeg.Do(params, ffmpeg.StreamInfo{
		TotalSeconds: input.Duration,
	}, progressCallback)
	if err != nil {
		return nil, err
	}

	err = os.Chmod(outputPath, os.ModePerm)
	if err != nil {
		return nil, err
	}

	return &common.MergeResult{
		Path: outputPath,
	}, nil
}

// mergeItemToStereoStream takes a merge input item and returns a string that can be used in a filter_complex to merge the audio streams.
func mergeItemToStereoStream(index int, tag string, item common.MergeInputItem) (string, error) {
	info, _ := ffmpeg.ProbeFile(item.Path)
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
		params = append(params, "-i", utils.IsilonPathFix(i.Path))
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

	outputPath := filepath.Join(input.OutputDir, filepath.Clean(input.Title)+".wav")

	params = append(params, "-filter_complex", filterComplex, "-map", "[a]", "-y", outputPath)

	_, err := ffmpeg.Do(params, ffmpeg.StreamInfo{}, progressCallback)

	return &common.MergeResult{
		Path: outputPath,
	}, err
}

func MergeSubtitles(input common.MergeInput, progressCallback ffmpeg.ProgressCallback) (*common.MergeResult, error) {
	var files []string
	// for each file, extract the specified range and save the result to a file.
	for index, item := range input.Items {
		file := filepath.Join(input.WorkDir, fmt.Sprintf("%d.srt", index))
		path := utils.IsilonPathFix(item.Path)
		cmd := exec.Command("ffmpeg", "-i", path, "-ss", fmt.Sprintf("%f", item.Start), "-to", fmt.Sprintf("%f", item.End), "-y", file)

		_, err := utils.ExecuteCmd(cmd, nil)
		if err != nil {
			return nil, err
		}
		files = append(files, file)
	}

	// the files have to be present in a text file for ffmpeg to concatenate them.
	// #subtitles.txt
	// file /path/to/file/0.srt
	// file /path/to/file/1.srt
	var content string
	for _, f := range files {
		content += fmt.Sprintf("file '%s'\n", f)
	}

	subtitlesFile := filepath.Join(input.WorkDir, "subtitles.txt")

	err := os.WriteFile(subtitlesFile, []byte(content), os.ModePerm)
	if err != nil {
		return nil, err
	}

	outputPath := filepath.Join(input.OutputDir, filepath.Clean(input.Title)+".srt")

	params := []string{
		"-progress", "pipe:1",
		"-hide_banner",
		"-f", "concat",
		"-safe", "0",
		"-i", subtitlesFile,
		"-y",
		outputPath,
	}

	_, err = ffmpeg.Do(params, ffmpeg.StreamInfo{}, progressCallback)

	return &common.MergeResult{
		Path: outputPath,
	}, err
}
