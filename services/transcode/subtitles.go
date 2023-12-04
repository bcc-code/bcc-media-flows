package transcode

import (
	_ "embed"
	"os"
	"strings"

	"github.com/bcc-code/bccm-flows/paths"
	"github.com/bcc-code/bccm-flows/services/ffmpeg"
)

//go:embed subtitles.header.ass
var defaultSubtitleHeader string

func SubtitleBurnIn(videoFile, subtitleFile, outputPath paths.Path, progressCallback ffmpeg.ProgressCallback) (*paths.Path, error) {
	if subtitleFile.Ext() != ".ass" {
		out := subtitleFile.Dir().Append(subtitleFile.Base() + ".ass")
		_, err := ffmpeg.Do([]string{
			"-i", subtitleFile.Local(),
			out.Local(),
		}, ffmpeg.StreamInfo{}, nil)
		if err != nil {
			return nil, err
		}
		subtitleContents, err := os.ReadFile(out.Local())
		if err != nil {
			return nil, err
		}
		eventsTagPassed := false
		var lines []string
		for _, l := range strings.Split(string(subtitleContents), "\n") {
			if strings.HasPrefix(l, "[Events]") {
				eventsTagPassed = true
				continue
			}
			if !eventsTagPassed {
				continue
			}
			lines = append(lines, l)
		}

		err = os.WriteFile(out.Local(), []byte(defaultSubtitleHeader+"\n"+strings.Join(lines, "\n")), os.ModePerm)
		if err != nil {
			return nil, err
		}

		subtitleFile = out
	}

	params := []string{
		"-i", videoFile.Local(),
		"-vf", "ass=" + subtitleFile.Local(),
		"-c:a", "copy",
	}

	base := videoFile.Base()
	filename := base[0 : len(base)-len(videoFile.Ext())]

	output := outputPath.Append(filename + ".subs" + videoFile.Ext())

	params = append(params, output.Local())

	info, err := ffmpeg.GetStreamInfo(videoFile.Local())
	if err != nil {
		return nil, err
	}

	_, err = ffmpeg.Do(params, info, progressCallback)
	if err != nil {
		return nil, err
	}

	return &output, nil
}
