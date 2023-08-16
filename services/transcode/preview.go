package transcode

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type PreviewInput struct {
	FilePath  string
	OutputDir string
}

type PreviewResult struct {
	LowResolutionPath string
}

func Preview(input PreviewInput) (*PreviewResult, error) {
	encoder := os.Getenv("ENCODER")
	if encoder == "" {
		encoder = "hevc"
	}

	filename := filepath.Base(input.FilePath)
	filename += "_lowres.mp4"
	outputPath := filepath.Join(input.OutputDir, filename)

	commandParts := []string{
		"-hide_banner",
		"-loglevel",
		"+level",
		"-y",
		"-ac 2",
		"-ss 0.0",
		fmt.Sprintf("-i %s", input.FilePath),
		"-ss 0.0",
		fmt.Sprintf("-i %s", os.Getenv("PREVIEW_WATERMARK_PATH")),
		"-filter_complex sws_flags=bicubic;[0:v]split=1[VIDEO-main-.mp4];[VIDEO-main-.mp4]scale=-2:540,null[temp];[temp][1:v]overlay=0:0:eof_action=repeat[VIDEO-.mp4];[0:a:0]asplit=1[AUDIO-main-.mp4-0];[AUDIO-main-.mp4-0]aformat=channel_layouts=stereo[AUDIO-.mp4-0]",
		"-map [VIDEO-.mp4]",
		"-map [AUDIO-.mp4-0]",
		fmt.Sprintf("-c:v %s", encoder),
		"-c:a:0 aac",
		"-ar:a:0 44100",
		"-b:a:0 128k",
		outputPath,
	}

	command := strings.Join(commandParts, " ")

	cmd := exec.Command("ffmpeg", strings.Split(command, " ")...)

	stderr, _ := cmd.StderrPipe()
	stdout, _ := cmd.StdoutPipe()

	_ = cmd.Start()

	go func() {
		scanner := bufio.NewScanner(stderr)
		scanner.Split(bufio.ScanLines)
		for scanner.Scan() {
			fmt.Print(scanner.Text())
		}
	}()

	scanner := bufio.NewScanner(stdout)
	scanner.Split(bufio.ScanLines)
	for scanner.Scan() {
		line := scanner.Text()
		fmt.Println(line)
	}

	err := cmd.Wait()
	if err != nil {
		return nil, err
	}

	return &PreviewResult{
		LowResolutionPath: outputPath,
	}, nil
}
