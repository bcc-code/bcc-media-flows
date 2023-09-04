package transcode

import (
	"fmt"
	"github.com/bcc-code/bccm-flows/common"
	"github.com/bcc-code/bccm-flows/utils"
	"github.com/samber/lo"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func MergeVideo(input common.MergeInput, progressCallback func(Progress)) (*common.MergeResult, error) {
	var params []string

	for _, i := range input.Items {
		params = append(params, "-i", i.Path)
	}

	var filterComplex string

	for index, i := range input.Items {
		filterComplex += fmt.Sprintf("[%d:v] trim=start=%f:end=%f,setpts=PTS-STARTPTS [v%d];", index, i.Start, i.End, index)
	}

	filterComplex += " "

	for index := range input.Items {
		filterComplex += fmt.Sprintf("[v%d] ", index)
	}

	filterComplex += fmt.Sprintf("concat=n=%d:v=1:a=0 [v]", len(input.Items))

	outputPath := filepath.Join(input.OutputDir, filepath.Clean(input.Title)+".mkv")

	params = append(params, "-progress", "pipe:1", "-filter_complex", filterComplex, "-map", "[v]", "-y", outputPath)

	cmd := exec.Command("ffmpeg", params...)

	_, err := utils.ExecuteCmd(cmd, parseProgressCallback(nil, progressCallback))

	return &common.MergeResult{
		Path: outputPath,
	}, err
}

func mergeItemToStereoStream(index int, tag string, item common.MergeInputItem) (string, error) {
	info, _ := ProbeFile(item.Path)

	if info == nil || len(info.Streams) == 0 {
		return fmt.Sprintf("anullsrc=channel_layout=stereo[%s]", tag), nil
	}

	streams := lo.Map(item.Streams, func(i, _ int) FFProbeStream {
		s, _ := lo.Find(info.Streams, func(s FFProbeStream) bool {
			return s.Index == i
		})
		return s
	})

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
	streamString += fmt.Sprintf("amerge=inputs=%d[%s]", channels, tag)

	return streamString, nil
}

// MergeAudio merges MXF audio files into one stereo file.
func MergeAudio(input common.MergeInput, progressCallback func(Progress)) (*common.MergeResult, error) {
	var params []string

	for _, i := range input.Items {
		params = append(params, "-i", i.Path)
	}

	params = append(params, "-c:a", "pcm_s16le")

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

	outputPath := filepath.Join(input.OutputDir, filepath.Clean(input.Title)+".mka")

	params = append(params, "-filter_complex", filterComplex, "-map", "[a]", "-y", outputPath)

	fmt.Println(strings.Join(params, " "))

	cmd := exec.Command("ffmpeg", params...)

	_, err := utils.ExecuteCmd(cmd, parseProgressCallback(nil, progressCallback))

	return &common.MergeResult{
		Path: outputPath,
	}, err
}

func MergeSubtitles(input common.MergeInput) (*common.MergeResult, error) {
	var files []string
	for index, item := range input.Items {
		file := filepath.Join(input.WorkDir, fmt.Sprintf("%d.srt", index))
		cmd := exec.Command("ffmpeg", "-i", item.Path, "-ss", fmt.Sprintf("%f", item.Start), "-to", fmt.Sprintf("%f", item.End), "-y", file)

		_, err := utils.ExecuteCmd(cmd, nil)
		if err != nil {
			return nil, err
		}
		files = append(files, file)
	}
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
	cmd := exec.Command("ffmpeg", "-f", "concat", "-safe", "0", "-i", subtitlesFile, "-y", outputPath)
	_, err = utils.ExecuteCmd(cmd, nil)
	if err != nil {
		return nil, err
	}

	return &common.MergeResult{
		Path: outputPath,
	}, nil
}
