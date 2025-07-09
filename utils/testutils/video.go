package testutils

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/bcc-code/bcc-media-flows/paths"
)

type VideoGeneratorParams struct {
	Duration  float64
	FrameRate int
	Width     int
	Height    int
	SAR       string
	DAR       string
	Profile   string
}

func GenerateVideoFile(outFile paths.Path, videoParams VideoGeneratorParams) paths.Path {
	os.MkdirAll(outFile.Dir().Local(), 0755)
	args := []string{
		"-f", "lavfi",
		"-i", fmt.Sprintf("testsrc=size=%dx%d:rate=%d:duration=%f", videoParams.Width, videoParams.Height, videoParams.FrameRate, videoParams.Duration),
		"-vf", fmt.Sprintf("setsar=%s, setdar=%s", videoParams.SAR, videoParams.DAR),
		"-c:v", "prores_ks",
		"-profile:v", videoParams.Profile,
		"-y", outFile.Local(),
	}

	cmd := exec.Command("ffmpeg", args...)
	err := cmd.Run()
	if err != nil {
		panic(err)
	}

	return outFile
}

// GenerateSeparateAudioStreamsTestFile generates a test file with separate audio streams
// instead of merged tracks in one stream
func GenerateSeparateAudioStreamsTestFile(outFile paths.Path, audioTracks int, duration float64) paths.Path {
	os.MkdirAll(outFile.Dir().Local(), 0755)
	
	args := []string{}
	
	// Add audio inputs - each as a separate stream
	for i := 0; i < audioTracks; i++ {
		freq := 100 + (i * 100) // Different frequency for each track
		args = append(args, 
			"-f", "lavfi",
			"-i", fmt.Sprintf("sine=frequency=%d:duration=%f:sample_rate=48000", freq, duration),
		)
	}
	
	// Add video input
	args = append(args,
		"-f", "lavfi",
		"-i", fmt.Sprintf("testsrc=duration=%f:size=1920x1080:rate=50", duration),
	)
	
	// Map each audio input as separate stream (no merging)
	for i := 0; i < audioTracks; i++ {
		args = append(args, "-map", fmt.Sprintf("%d:a", i))
	}
	
	// Map video
	args = append(args, "-map", fmt.Sprintf("%d:v", audioTracks))
	
	// Codec settings
	args = append(args,
		"-c:v", "libx264",
		"-c:a", "pcm_s16le",
		"-y", outFile.Local(),
	)

	cmd := exec.Command("ffmpeg", args...)
	err := cmd.Run()
	if err != nil {
		panic(err)
	}

	return outFile
}
