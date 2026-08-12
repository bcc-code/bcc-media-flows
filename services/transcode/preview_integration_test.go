//go:build integration

// Tests in this file drive real ffmpeg over generated media, so a single case costs
// tens of seconds and they are excluded from the default test run. The unit tests in
// preview_test.go cover the pure filter-building logic and stay fast.
//
// Run these with: go test -tags=integration ./services/transcode/...
package transcode

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/bcc-code/bcc-media-flows/paths"
	"github.com/bcc-code/bcc-media-flows/services/ffmpeg"
	"github.com/bcc-code/bcc-media-flows/utils/testutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGrowingPreview_VUMeters(t *testing.T) {
	os.MkdirAll("testdata/generated", 0755)

	inputFile := "testdata/generated/testsrc_growing_4streams.mxf"
	p, err := paths.Parse(inputFile)
	require.NoError(t, err)
	testutils.GenerateStreamableMXFTestFile(p, 4, 5.0)

	tempDir := t.TempDir()
	destFile := filepath.Join(tempDir, "growing_preview.mp4")

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- GrowingPreview(ctx, GrowingPreviewInput{
			FilePath:        inputFile,
			TempDir:         tempDir,
			DestinationFile: destFile,
			WatermarkPath:   "testdata/test_overlay.png",
		}, func(ctx context.Context, duration time.Duration) {})
	}()

	// Give ffmpeg time to consume the static file, then stop the tail
	time.Sleep(10 * time.Second)
	cancel()

	select {
	case err := <-done:
		// Previously discarded. Cancellation is how every live ingest ends normally, so
		// it must not be reported as a failure — and asserting it here also guards
		// against killing ffmpeg outright, which would truncate the final segment.
		require.NoError(t, err, "cancellation is the normal end of a live ingest")
	case <-time.After(60 * time.Second):
		t.Fatal("GrowingPreview did not exit after context cancellation")
	}

	stat, err := os.Stat(destFile)
	require.NoError(t, err, "destination file should exist")
	require.True(t, stat.Size() > 1000, "destination file should not be empty")

	info, err := ffmpeg.ProbeFile(destFile)
	require.NoError(t, err)
	var videoStreams, audioStreams int
	for _, stream := range info.Streams {
		switch stream.CodecType {
		case "video":
			videoStreams++
		case "audio":
			audioStreams++
		}
	}
	assert.Equal(t, 1, videoStreams)
	assert.Equal(t, 1, audioStreams)
}

func TestPreview_VUMeters_MultipleAudioTracks(t *testing.T) {
	t.Parallel()
	trackCounts := []int{1, 2, 4, 16}
	os.MkdirAll("testdata/generated", 0755)

	for _, n := range trackCounts {
		t.Run(fmt.Sprintf("audio_tracks_%d", n), func(t *testing.T) {
			inputFile := filepath.Join("testdata/generated", fmt.Sprintf("testsrc_%dtracks.mov", n))
			outputDir := "testdata/generated"
			p, err := paths.Parse(inputFile)
			require.NoError(t, err)
			testutils.GenerateSoftronTestFile(p, n, 2.0)

			previewInput := PreviewInput{
				FilePath:      inputFile,
				OutputDir:     outputDir,
				WatermarkPath: "testdata/test_overlay.png",
			}

			result, err := Preview(previewInput, nil)
			require.NoError(t, err, "Preview should succeed for %d tracks", n)
			require.NotNil(t, result)
			stat, err := os.Stat(result.LowResolutionPath)
			require.NoError(t, err)
			require.True(t, stat.Size() > 1000, "Preview output should not be empty for %d tracks", n)
		})
	}
}

func TestPreview_VUMeters_SeparateAudioStreams(t *testing.T) {
	t.Parallel()
	trackCounts := []int{1, 2, 4, 8}
	os.MkdirAll("testdata/generated", 0755)

	for _, n := range trackCounts {
		t.Run(fmt.Sprintf("separate_streams_%d", n), func(t *testing.T) {
			inputFile := filepath.Join("testdata/generated", fmt.Sprintf("testsrc_separate_%dstreams.mov", n))
			outputDir := "testdata/generated"
			p, err := paths.Parse(inputFile)
			require.NoError(t, err)
			testutils.GenerateSeparateAudioStreamsTestFile(p, n, 2.0)

			previewInput := PreviewInput{
				FilePath:      inputFile,
				OutputDir:     outputDir,
				WatermarkPath: "testdata/test_overlay.png",
			}

			result, err := Preview(previewInput, nil)
			require.NoError(t, err, "Preview should succeed for %d separate streams", n)
			require.NotNil(t, result)
			stat, err := os.Stat(result.LowResolutionPath)
			require.NoError(t, err)
			require.True(t, stat.Size() > 1000, "Preview output should not be empty for %d separate streams", n)
		})
	}
}
