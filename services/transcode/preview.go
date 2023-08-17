package transcode

import (
	"errors"
	"fmt"
	"github.com/bcc-code/bccm-flows/utils"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

type PreviewInput struct {
	FilePath  string
	OutputDir string
}

type PreviewResult struct {
	LowResolutionPath string
}

var previewWatermarkPath = os.Getenv("PREVIEW_WATERMARK_PATH")

func Preview(input PreviewInput, progressCallback func(float64)) (*PreviewResult, error) {
	encoder := os.Getenv("ENCODER")
	if encoder == "" {
		encoder = "hevc"
	}

	info, err := ProbeFile(input.FilePath)
	if err != nil {
		return nil, err
	}

	var hasVideo bool
	var hasAudio bool
	for _, stream := range info.Streams {
		if stream.CodecType == "video" {
			hasVideo = true
		} else if stream.CodecType == "audio" {
			hasAudio = true
		}
	}

	filename := filepath.Base(input.FilePath)

	if hasVideo {
		filename += "_lowres.mp4"
	} else if hasAudio {
		filename += "_lowaudio.mp4"
	} else {
		return nil, errors.New("input file not supported")
	}

	outputPath := filepath.Join(input.OutputDir, filename)

	var commandParts []string
	if hasVideo {
		fmt.Println("Transcoding video file")
		commandParts = []string{
			"-hide_banner",
			"-loglevel",
			"+level",
			"-progress pipe:1",
			"-y",
			"-ac 2",
			"-ss 0.0",
			fmt.Sprintf("-i %s", input.FilePath),
			"-ss 0.0",
			fmt.Sprintf("-i %s", previewWatermarkPath),
			"-filter_complex sws_flags=bicubic;[0:v]split=1[VIDEO-main-.mp4];[VIDEO-main-.mp4]scale=-2:540,null[temp];[temp][1:v]overlay=0:0:eof_action=repeat[VIDEO-.mp4];[0:a:0]asplit=1[AUDIO-main-.mp4-0];[AUDIO-main-.mp4-0]aformat=channel_layouts=stereo[AUDIO-.mp4-0]",
			"-map [VIDEO-.mp4]",
			"-map [AUDIO-.mp4-0]",
			fmt.Sprintf("-c:v %s", encoder),
			"-c:a:0 aac",
			"-ar:a:0 44100",
			"-b:a:0 128k",
			outputPath,
		}
	} else if hasAudio {
		fmt.Println("Transcoding audio file")
		commandParts = []string{
			"-hide_banner",
			"-loglevel",
			"+level",
			"-progress pipe:1",
			"-y",
			"-ss 0.0",
			fmt.Sprintf("-i %s", input.FilePath),
			"-filter_complex sws_flags=bicubic;[0:a:0]asplit=1[AUDIO-main-.mp4-0];[AUDIO-main-.mp4-0]aformat=channel_layouts=stereo[AUDIO-.mp4-0]",
			"-map [AUDIO-.mp4-0]",
			"-vn",
			"-c:a:0 aac",
			"-ar:a:0 44100",
			"-b:a:0 128k",
			outputPath,
		}
	}

	command := strings.Join(commandParts, " ")

	fmt.Println(command)

	cmd := exec.Command("ffmpeg", strings.Split(command, " ")...)

	totalFrames, _ := strconv.ParseFloat(info.Streams[0].NbFrames, 64)

	callback := func(line string) {
		parts := strings.Split(line, "=")

		if len(parts) != 2 {
			return
		}

		if parts[0] == "frame" {
			frame, _ := strconv.ParseFloat(parts[1], 64)
			if frame == 0 {
				progressCallback(0)
			} else {
				progressCallback(frame / totalFrames)
			}
		}
		if parts[0] == "progress" {
			// Audio doesn't report progress in a conceivable way, so just return 1 on complete
			if parts[1] == "end" {
				progressCallback(1)
			}
		}
	}

	_, err = utils.ExecuteCmd(cmd, callback)
	if err != nil {
		return nil, err
	}

	return &PreviewResult{
		LowResolutionPath: outputPath,
	}, nil
}
