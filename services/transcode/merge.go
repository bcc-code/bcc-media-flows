package transcode

import (
	"cmp"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/bcc-code/bcc-media-flows/paths"

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
	var filterComplex string

	for index, i := range input.Items {
		interlaceString := ""

		info, err := ffmpeg.GetStreamInfo(i.Path.Local())
		if err != nil {
			return nil, err
		}
		if !info.Progressive {
			interlaceString = ",yadif"
		}

		// Add the video stream and timestamps to the filter, with setpts to let the transcoder know to continue the timestamp from the previous file.
		filterComplex += fmt.Sprintf("[%d:v]trim=start=%f:end=%f,setpts=PTS-STARTPTS%s[v%d];", index, i.Start, i.End, interlaceString, index)
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
	)

	_, err = runMergeJob(input, outputFilePath, params, progressCallback)
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

	// Restrict stream lookup to audio streams. Vidispine's EssenceStreamID
	// does not always line up with ffmpeg's 0-based stream Index when
	// non-audio streams precede audio in the container, so matching against
	// info.Streams directly can pick up a video or data stream.
	audioStreams := info.AudioStreams()

	var streams []ffmpeg.FFProbeStream

	for _, stream := range item.Streams {
		s, found := lo.Find(audioStreams, func(s ffmpeg.FFProbeStream) bool {
			return s.Index == stream.StreamID
		})
		if found {
			streams = append(streams, s)
		}
	}

	if len(streams) == 0 {
		s, found := lo.Find(audioStreams, func(s ffmpeg.FFProbeStream) bool {
			return s.Channels == 2
		})
		if found {
			streams = append(streams, s)
		}
	}

	var streamString string
	channels := 0
	for _, stream := range streams {
		if stream.Channels == 2 {
			return fmt.Sprintf("[%d:%d]aselect[%s]", index, stream.Index, tag), nil
		} else if stream.Channels == 64 && len(item.Streams) == 2 {
			streamString += fmt.Sprintf("[0:a]pan=stereo|c0=c%d|c1=c%d[%s]", item.Streams[0].ChannelID, item.Streams[1].ChannelID, tag)
			return streamString, nil
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
		"-c:a", "pcm_s16le",
	}

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

	params = append(params, "-filter_complex", filterComplex, "-map", "[a]")

	_, err := runMergeJob(input, outputFilePath, params, progressCallback)
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

// MergeSubtitlesByOffset merges subtitles based on a specified offset
//
// This is used for example when you have several movies played in a feast.
// this way the offset indicates the offset from the start of the event, and the subtitles will be placed there
func MergeSubtitlesByOffset(input common.MergeInput, progressCallback ffmpeg.ProgressCallback) (*common.MergeResult, error) {
	var files []string

	for index, item := range input.Items {
		fileOut := filepath.Join(input.WorkDir.Local(), fmt.Sprintf("%s-%d-out.srt", input.Title, index))
		path := item.Path.Local()

		cmd := exec.Command("ffmpeg",
			"-hide_banner",
			"-itsoffset", fmt.Sprintf("%f", item.StartOffset),
			"-i", path,
			"-y", fileOut,
		)
		_, err := utils.ExecuteCmd(cmd, nil)
		if err != nil {
			return nil, err
		}

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

	err := os.WriteFile(subtitlesFile, []byte(content), ffmpeg.OutputFileMode)
	if err != nil {
		return nil, err
	}

	for _, f := range files {
		err = ensureValidSrtFile(f)
		if err != nil {
			return nil, err
		}
	}

	concatStr := fmt.Sprintf("concat:%s", strings.Join(files, "|"))

	outputFilePath := filepath.Join(input.OutputDir.Local(), filepath.Clean(input.Title)+".srt")
	// The input is ffmpeg's concat: pseudo-protocol rather than one of the merge
	// items, so this does not go through runMergeJob.
	_, err = ffmpeg.Run(ffmpeg.Job{
		Input:  concatStr,
		Output: outputFilePath,
		Args:   []string{"-c", "copy"},
		Info:   &ffmpeg.StreamInfo{},
	}, progressCallback)
	if err != nil {
		return nil, err
	}

	err = ensureValidSrtFile(outputFilePath)
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

// MergeSubtitles does the merging of subtitles for the mormal mediabanken export
//
// Each item's cues between Start and End are moved so that they follow the
// previous items. This is done here rather than with ffmpeg: whether ffmpeg
// keeps the source times on an output -ss for subtitles or rebases them to the
// -ss point differs between builds, and guessing wrong moved every cue so the
// first one started at 00:00:00.
//
// Like ffmpeg's cut, a cue belongs to an item when it starts inside it, so a
// cue straddling the in-point is dropped and one straddling the out-point keeps
// its end.
func MergeSubtitles(input common.MergeInput, _ ffmpeg.ProgressCallback) (*common.MergeResult, error) {
	var merged []srtCue

	var startAt int64
	for _, item := range input.Items {
		data, err := os.ReadFile(item.Path.Local())
		if err != nil {
			return nil, err
		}

		cues, err := parseSRT(data)
		if err != nil {
			return nil, fmt.Errorf("parsing subtitles %s: %w", item.Path.Local(), err)
		}

		// The demuxer orders cues by start, and Whisper output is not always in order.
		slices.SortStableFunc(cues, func(a, b srtCue) int {
			return cmp.Compare(a.Start, b.Start)
		})
		repairBackwardCues(cues)

		itemStart := secondsToMillis(item.Start)
		itemEnd := secondsToMillis(item.End)
		shift := startAt - itemStart

		for _, cue := range cues {
			if cue.Start < itemStart || cue.Start >= itemEnd {
				continue
			}
			cue.Start += shift
			cue.End += shift
			merged = append(merged, cue)
		}

		startAt += itemEnd - itemStart
	}

	if err := os.MkdirAll(input.OutputDir.Local(), ffmpeg.OutputDirMode); err != nil {
		return nil, err
	}

	outputFilePath := filepath.Join(input.OutputDir.Local(), filepath.Clean(input.Title)+".srt")
	err := os.WriteFile(outputFilePath, writeSRT(merged), ffmpeg.OutputFileMode)
	if err != nil {
		return nil, err
	}

	err = ensureValidSrtFile(outputFilePath)
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

func ensureValidSrtFile(f string) error {
	info, err := os.Stat(f)
	if err != nil {
		return err
	}
	if info.Size() == 0 {
		err = os.WriteFile(f, []byte("1\n00:00:00,000 --> 00:00:00,000\n"), ffmpeg.OutputFileMode)
		if err != nil {
			return err
		}
	}

	return nil
}

// runMergeJob runs a merge: the inputs are the merge items in order — the
// filter graphs refer to them by that index — and progress is measured against
// the merged duration rather than any one input.
func runMergeJob(
	input common.MergeInput,
	output string,
	args []string,
	cb ffmpeg.ProgressCallback,
) (ffmpeg.StreamInfo, error) {
	if len(input.Items) == 0 {
		return ffmpeg.StreamInfo{}, fmt.Errorf("nothing to merge into %s", output)
	}

	inputs := make([]string, 0, len(input.Items))
	for _, item := range input.Items {
		inputs = append(inputs, item.Path.Local())
	}

	return ffmpeg.Run(ffmpeg.Job{
		Input:       inputs[0],
		ExtraInputs: ffmpeg.FileInputs(inputs[1:]),
		Output:      output,
		Args:        args,
		Info:        &ffmpeg.StreamInfo{TotalSeconds: input.Duration},
	}, cb)
}
