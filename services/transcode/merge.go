package transcode

import (
	"fmt"
	"github.com/bcc-code/bccm-flows/utils"
	"github.com/samber/lo"
	"os/exec"
	"path/filepath"
	"strings"
)

type MergeInputItem struct {
	Path    string
	Start   float64
	End     float64
	Streams []int
}

type MergeInput struct {
	Title     string
	Items     []MergeInputItem
	OutputDir string
}

func MergeVideo(input MergeInput) (*EncodeResult, error) {
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

	params = append(params, "-filter_complex", filterComplex, "-map", "[v]", "-y", outputPath)

	cmd := exec.Command("ffmpeg", params...)

	_, err := utils.ExecuteCmd(cmd, nil)

	return nil, err
}

func mergeItemToStereoStream(index int, tag string, item MergeInputItem) (string, error) {
	info, err := ProbeFile(item.Path)
	if err != nil {
		return "", err
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
func MergeAudio(input MergeInput) (*EncodeResult, error) {
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

	_, err := utils.ExecuteCmd(cmd, nil)

	return &EncodeResult{
		Path: outputPath,
	}, err
}
