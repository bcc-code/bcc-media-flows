package transcode

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	bccmflows "github.com/bcc-code/bcc-media-flows"
	"github.com/bcc-code/bcc-media-flows/environment"
	"github.com/bcc-code/bcc-media-flows/services/ffmpeg"
)

type PreviewInput struct {
	FilePath      string
	OutputDir     string
	WatermarkPath string
}

type GrowingPreviewInput struct {
	FilePath        string
	TempDir         string
	DestinationFile string
	WatermarkPath   string
}

type PreviewResult struct {
	LowResolutionPath string
	AudioOnly         bool
}

type AudioPreviewResult struct {
	AudioTracks map[string]string
}

var previewWatermarkPath = environment.GetIsilonPrefix() + "/system/graphics/LOGO_BTV_Preview_960-540.mov"

// ErrUnknownAudioChannelFormat means the audio-channel layout wasn't recognized (not MU1/MU2),
// so per-language audio previews can't be built. Callers should skip audio preview but may still
// produce the video preview.
var ErrUnknownAudioChannelFormat = errors.New("unknown format of audio channels")

type audioPreviewData struct {
	FFMPEGParams []string
	LanguageMap  map[string]string
}

// buildVUMeterFilters generates ffmpeg filter steps for compact VU meters for each audio track.
func buildVUMeterFilters(audioTracks int, trcPrefix string, scaleFilter string) (string, string) {
	meterW := 200
	meterH := 20
	meterAlpha := 0.5
	spacing := 10
	parts := []string{fmt.Sprintf("[0:v]%s%s[vmain]", trcPrefix, scaleFilter)}
	lastVid := "[vmain]"
	for i := 0; i < audioTracks; i++ {
		y := 10 + i*(meterH+spacing)
		parts = append(parts,
			fmt.Sprintf("[0:a:%d]showvolume=w=%d:h=%d:p=%.2f:t=1,format=rgba[vum%d]", i, meterW, meterH, meterAlpha, i),
			fmt.Sprintf("%s[vum%d]overlay=x=10:y=%d[tmp%d]", lastVid, i, y, i),
		)
		lastVid = fmt.Sprintf("[tmp%d]", i)
	}
	return strings.Join(parts, ";"), lastVid
}

// buildPreviewAudioFilter returns the stereo downmix filter labeled [AUDIO-.mp4-0].
func buildPreviewAudioFilter(audioTracks int) string {
	if audioTracks == 1 {
		// Single stream: duplicate to both channels
		return "[0:a:0]aformat=channel_layouts=stereo[AUDIO-.mp4-0]"
	} else if audioTracks >= 2 {
		// Multiple streams: stream 1 to left, stream 2 to right
		return "[0:a:0][0:a:1]amerge=inputs=2,pan=stereo|c0<c0|c1<c1[AUDIO-.mp4-0]"
	}
	// Fallback for edge cases
	return "[0:a]aformat=channel_layouts=stereo[AUDIO-.mp4-0]"
}

// buildGrowingPreviewFilter returns the full -filter_complex for GrowingPreview.
// audioTracks <= 0 (probe failed / no audio detected) yields the legacy filter without VU meters.
func buildGrowingPreviewFilter(audioTracks int, trcPrefix string) string {
	if audioTracks <= 0 {
		return "sws_flags=bicubic;[0:v]split=1[VIDEO-main-.mp4];[VIDEO-main-.mp4]scale=-2:540,null[temp];[temp][1:v]overlay=0:0:eof_action=repeat[VIDEO-.mp4];[0:a]aformat=channel_layouts=stereo[AUDIO-.mp4-0]"
	}
	vuFilters, lastVid := buildVUMeterFilters(audioTracks, trcPrefix, "scale=-2:540")
	return fmt.Sprintf(
		"sws_flags=bicubic;%s;%s[1:v]overlay=0:0:eof_action=repeat[VIDEO-.mp4];%s",
		vuFilters, lastVid, buildPreviewAudioFilter(audioTracks),
	)
}

func prepareAudioPreview(isMU1, isMU2 bool, fileInfo *ffmpeg.FFProbeResult, inputFile, outputDir string) (*audioPreviewData, error) {
	audioStreams := fileInfo.AudioStreams()

	fileMap := map[string]string{}
	filterParts := []string{}
	audioMap := []string{}

	if len(audioStreams) == 16 {
		if isMU1 {
			for i, l := range bccmflows.LanguagesByMU1 {
				if l.MU1ChannelStart != i {
					continue // skip duplicated languages
				}

				fileName := filepath.Join(outputDir, fmt.Sprintf("%d.%s.aac", i, l.ISO6391))

				if l.MU1ChannelCount == 1 {
					filterParts = append(filterParts, fmt.Sprintf("[0:%d]acopy[a%d]", l.MU1ChannelStart, i))
				} else {
					filterParts = append(filterParts, fmt.Sprintf("[0:%d][0:%d]amerge=inputs=2[a%d]", l.MU1ChannelStart, l.MU1ChannelStart+1, i))
				}

				audioMap = append(audioMap, "-map", fmt.Sprintf("[a%d]", i), fileName)

				fileMap[l.ISO6391] = fileName
			}
		} else if isMU2 {
			for i, l := range bccmflows.LanguagesByMU2 {
				if l.MU2ChannelStart != i {
					continue // skip duplicated languages
				}

				fileName := filepath.Join(outputDir, fmt.Sprintf("%d.%s.aac", i, l.ISO6391))

				if l.MU2ChannelCount == 1 {
					filterParts = append(filterParts, fmt.Sprintf("[0:%d]acopy[a%d]", l.MU2ChannelStart, i))
				} else {
					filterParts = append(filterParts, fmt.Sprintf("[0:%d][0:%d]amerge=inputs=2[a%d]", l.MU2ChannelStart, l.MU2ChannelStart+1, i))
				}

				audioMap = append(audioMap, "-map", fmt.Sprintf("[a%d]", i), fileName)
				fileMap[l.ISO6391] = fileName
			}
		} else {
			return nil, ErrUnknownAudioChannelFormat
		}

	} else if len(audioStreams) == 1 && audioStreams[0].Channels == 64 {
		for i, l := range bccmflows.LanguageBySoftron {
			fileName := filepath.Join(outputDir, fmt.Sprintf("%d.%s.aac", i, l.ISO6391))
			filterParts = append(filterParts, fmt.Sprintf("[0:%d]pan=stereo|c0=c%d|c1=c%d[a%d]", audioStreams[0].Index, l.SoftronStartCh, l.SoftronStartCh+1, i))
			audioMap = append(audioMap, "-map", fmt.Sprintf("[a%d]", i), fileName)
			fileMap[l.ISO6391] = fileName
		}
	} else {
		return nil, nil
	}

	// This is here to stabilize the string for unit tests
	sort.Strings(filterParts)

	args := []string{
		"-i", inputFile,
		"-c:a", "aac", "-b:a", "64k", "-ar", "44100", "-ac", "2", "-profile:a", "aac_low",
		"-filter_complex", strings.Join(filterParts, ";"),
		"-y",
	}

	args = append(args, audioMap...)

	return &audioPreviewData{
		LanguageMap:  fileMap,
		FFMPEGParams: args,
	}, nil
}

func AudioPreview(input PreviewInput, progressCallback ffmpeg.ProgressCallback) (*AudioPreviewResult, error) {
	out := &AudioPreviewResult{}

	isMU1 := strings.Contains(input.FilePath, "_MU1")
	isMU2 := strings.Contains(input.FilePath, "_MU2")

	info, err := ffmpeg.ProbeFile(input.FilePath)
	if err != nil {
		return nil, err
	}

	previewData, err := prepareAudioPreview(isMU1, isMU2, info, input.FilePath, input.OutputDir)
	if err != nil {
		return nil, err
	}

	if previewData == nil {
		return out, nil
	}

	// One output per language, so there is no single output for Run or RunArgs to
	// create a directory for and chmod.
	_, err = ffmpeg.Do(previewData.FFMPEGParams, ffmpeg.ProbeResultToInfo(info), progressCallback)
	if err != nil {
		return nil, err
	}

	out.AudioTracks = previewData.LanguageMap

	return out, nil
}

func Preview(input PreviewInput, progressCallback ffmpeg.ProgressCallback) (*PreviewResult, error) {
	encoder := os.Getenv("H264_ENCODER")
	if encoder == "" {
		encoder = "libx264"
	}

	watermark := input.WatermarkPath
	if watermark == "" {
		watermark = previewWatermarkPath
	}

	info, err := ffmpeg.ProbeFile(input.FilePath)
	if err != nil {
		return nil, err
	}

	audioTracks := len(info.AudioStreams())
	hasAudio := audioTracks > 0
	hasVideo := len(info.VideoStreams()) > 0

	var trcPrefix string
	if trcFix := ffmpeg.NormalizeColorTRCFilter(ffmpeg.ProbeResultToInfo(info)); trcFix != "" {
		trcPrefix = trcFix + ","
	}

	filename := filepath.Base(input.FilePath)[:len(filepath.Base(input.FilePath))-len(filepath.Ext(input.FilePath))]
	if hasVideo {
		filename += "_lowres.mp4"
	} else if hasAudio {
		filename += "_lowaudio.mp4"
	} else {
		return nil, errors.New("input file not supported")
	}

	outputPath := filepath.Join(input.OutputDir, filename)

	var params = []string{
		"-progress", "pipe:1",
		"-hide_banner",
		"-loglevel",
		"+level",
		"-y",
	}

	if hasVideo && !hasAudio {
		params = append(params,
			"-i", input.FilePath,
			"-ss", "0.0",
			"-i", watermark,
			"-filter_complex", fmt.Sprintf("sws_flags=bicubic;[0:v]%ssplit=1[VIDEO-main-.mp4];[VIDEO-main-.mp4]scale=-2:540,null[temp];[temp][1:v]overlay=0:0:eof_action=repeat[VIDEO-.mp4]", trcPrefix),
			"-map", "[VIDEO-.mp4]",
			"-c:v", encoder,
		)
	} else if hasVideo && hasAudio {
		// VU meters + watermark
		params = append(params,
			"-ac", "2",
			"-ss", "0.0",
			"-i", input.FilePath,
			"-ss", "0.0",
			"-i", watermark,
		)
		vuFilters, lastVid := buildVUMeterFilters(audioTracks, trcPrefix, "scale=1280:720")
		// Compose filter graph: scale, vumeters, watermark, stereo audio
		filter := fmt.Sprintf(
			"sws_flags=bicubic;%s;[1:v]scale=1280:720[wm];%s[wm]overlay=0:0:eof_action=repeat[VIDEO-.mp4];%s",
			vuFilters, lastVid, buildPreviewAudioFilter(audioTracks),
		)
		params = append(params,
			"-filter_complex", filter,
			"-map", "[VIDEO-.mp4]",
			"-map", "[AUDIO-.mp4-0]",
			"-c:v", encoder,
		)
	} else if hasAudio {
		params = append(params,
			"-ss", "0.0",
			"-i", input.FilePath,
			"-filter_complex", "sws_flags=bicubic;[0:a:0]asplit=1[AUDIO-main-.mp4-0];[AUDIO-main-.mp4-0]aformat=channel_layouts=stereo[AUDIO-.mp4-0]",
			"-map", "[AUDIO-.mp4-0]",
			"-vn",
		)
	}

	params = append(params,
		"-c:a:0", "aac",
		"-ar:a:0", "44100",
		"-b:a:0", "128k",
		outputPath,
	)

	// RunArgs rather than a Job: which files are inputs depends on whether the
	// source has video, audio or both, and -ss applies to the watermark input it
	// precedes.
	err = ffmpeg.RunArgs(params, outputPath, ffmpeg.ProbeResultToInfo(info), progressCallback)
	if err != nil {
		return nil, err
	}

	return &PreviewResult{
		LowResolutionPath: outputPath,
		AudioOnly:         !hasVideo && hasAudio,
	}, nil
}

// GrowingPreview creates a preview for a growing video
//
// The preview is created by tailing the video file and piping it to ffmpeg.
// Since this function does not know when the file is finished, it will continue
// to tail the file until it's context is cancelled.
func GrowingPreview(ctx context.Context, input GrowingPreviewInput, heartbeater func(ctx context.Context, duration time.Duration)) error {
	watermark := input.WatermarkPath
	if watermark == "" {
		watermark = previewWatermarkPath
	}

	// Probe the growing file to determine the audio track count for the VU meters.
	// The stream layout lives in the file header, so probing a partial file works.
	// On failure we fall back to the legacy filter without VU meters rather than
	// failing the live ingest.
	audioTracks := 0
	trcPrefix := ""
	if info, err := ffmpeg.ProbeFile(input.FilePath); err != nil {
		fmt.Printf("growing preview: probe failed, falling back to filter without VU meters: %v\n", err)
	} else {
		audioTracks = len(info.AudioStreams())
		if trcFix := ffmpeg.NormalizeColorTRCFilter(ffmpeg.ProbeResultToInfo(info)); trcFix != "" {
			trcPrefix = trcFix + ","
		}
	}

	tailCmd := exec.CommandContext(ctx, "tail", "-c", "+1", "-f", input.FilePath)

	// Context-bound so a cancelled or timed-out activity cannot leave ffmpeg running.
	// This matters on retry: a new attempt restarts tail from byte 0, and an orphan
	// from the previous attempt would keep writing into the same TempDir, so two
	// ffmpegs would be producing the same HLS segments.
	//
	// Cancel is deliberately a no-op instead of CommandContext's default SIGKILL.
	// Cancellation kills tail, ffmpeg then sees EOF on stdin and finalises the HLS
	// playlist cleanly; killing it outright would risk truncating the final segment.
	// WaitDelay bounds how long Wait may block after cancellation before force-killing,
	// so a wedged ffmpeg cannot hang the activity indefinitely.
	//
	// Note the documented consequence: because Cancel returns nil rather than
	// os.ErrProcessDone, Wait reports the context error even when ffmpeg exits
	// successfully. That is why the cancellation path below ignores Wait's result.
	ffmpegCmd := exec.CommandContext(ctx, "ffmpeg",
		"-i", "pipe:0", // Input from stdin
		"-i", watermark,
		"-c:v", "libx264", // Video codec: H.264
		"-c:a", "aac", // Audio codec: AAC
		"-filter_complex", buildGrowingPreviewFilter(audioTracks, trcPrefix),
		"-map", "[VIDEO-.mp4]",
		"-map", "[AUDIO-.mp4-0]",
		"-strict", "experimental", // Allow experimental codecs
		"-f", "hls", // Format HLS
		"-hls_time", "60", // 60-second segments
		"-hls_list_size", "0", // Unlimited entries in the playlist
		"-hls_segment_filename", filepath.Join(input.TempDir, "segment_%03d.ts"), // Segment file names
		"-y", filepath.Join(input.TempDir, "playlist.m3u8")) // Output playlist file

	ffmpegCmd.Cancel = func() error { return nil }
	ffmpegCmd.WaitDelay = 2 * time.Minute

	// Create a pipe between the two commands
	pipe, err := tailCmd.StdoutPipe()
	if err != nil {
		// Must return rather than exit: this runs inside a Temporal activity, so
		// terminating the process would kill every other activity on the worker without
		// reporting an error, leaving them to fail later on a heartbeat timeout.
		return fmt.Errorf("could not create tail stdout pipe: %w", err)
	}
	ffmpegCmd.Stdin = pipe

	// Set output for ffmpeg. Stderr is teed so operators keep seeing it in the worker
	// log while the tail is also available to quote in an error. Bounded on purpose:
	// activity errors are stored in the workflow history, and a decode-warning storm on
	// a multi-hour ingest would otherwise put megabytes there.
	ffmpegCmd.Stdout = os.Stdout
	stderrTail := &boundedTailWriter{max: maxFFmpegErrorTail}
	ffmpegCmd.Stderr = io.MultiWriter(os.Stderr, stderrTail)

	fmt.Printf("Executing tail command: %s\n", strings.Join(tailCmd.Args, " "))
	fmt.Printf("Executing ffmpeg command: %s\n", strings.Join(ffmpegCmd.Args, " "))

	// Start tail command
	if err := tailCmd.Start(); err != nil {
		return fmt.Errorf("error starting tail: %w", err)
	}

	// Reaps tail and closes the pipe's read end. Skipping this leaks a descriptor and
	// leaves a zombie per call: StdoutPipe registers the read end in
	// tailCmd.parentIOPipes, which only Wait closes, and handing it to ffmpegCmd.Stdin
	// does not transfer ownership because exec returns a caller-supplied *os.File as-is.
	defer func() { _ = tailCmd.Wait() }()

	// Start ffmpeg command
	if err := ffmpegCmd.Start(); err != nil {
		return fmt.Errorf("error starting ffmpeg: %w\nCommand: %s", err, strings.Join(ffmpegCmd.Args, " "))
	}

	ffmpegDone := make(chan error, 1)
	go func() { ffmpegDone <- ffmpegCmd.Wait() }()

	start := time.Now()
	for {
		select {
		case waitErr := <-ffmpegDone:
			// ffmpeg stopped while the file is still growing, so no more preview is
			// coming. Without this case the activity would carry on heartbeating and
			// remuxing a stale playlist until its 8 hour timeout.
			if muxErr := muxFinishedPreview(input.TempDir, input.DestinationFile); muxErr != nil {
				fmt.Println(muxErr)
			}
			return fmt.Errorf("ffmpeg exited before the ingest finished: %w\nffmpeg stderr:\n%s",
				waitErr, stderrTail.String())

		case <-ctx.Done():
			// Expected path: the workflow cancels this activity once the ingest is
			// complete. Killing tail closes ffmpeg's stdin, which lets it finalise the
			// playlist. Bounded by WaitDelay.
			<-ffmpegDone

			// Mux once more after ffmpeg has exited so the final HLS segment is included.
			if muxErr := muxFinishedPreview(input.TempDir, input.DestinationFile); muxErr != nil {
				fmt.Println(muxErr)
			}

			// ffmpeg's exit status here describes the shutdown, not the outcome — the
			// deliverable is the muxed file. Returning it would make the normal end of
			// every live ingest look like a failure.
			return nil

		case <-time.After(60 * time.Second):
			heartbeater(ctx, time.Since(start))

			if muxErr := muxFinishedPreview(input.TempDir, input.DestinationFile); muxErr != nil {
				// Expected mid-ingest: this mux can race ffmpeg rewriting the playlist.
				fmt.Println(muxErr)
			}
		}
	}
}

// maxFFmpegErrorTail is how much of ffmpeg's stderr is quoted back in an error.
const maxFFmpegErrorTail = 4096

// boundedTailWriter keeps only the last max bytes written to it, so a long-running
// command's stderr can be quoted in an error without it growing without bound.
type boundedTailWriter struct {
	buf []byte
	max int
}

func (w *boundedTailWriter) Write(p []byte) (int, error) {
	w.buf = append(w.buf, p...)
	if len(w.buf) > w.max {
		w.buf = w.buf[len(w.buf)-w.max:]
	}
	return len(p), nil
}

func (w *boundedTailWriter) String() string {
	return string(w.buf)
}

func muxFinishedPreview(inputFolder, outputFile string) error {
	// Copy the playlist and append the end tag
	input, err := os.ReadFile(filepath.Join(inputFolder, "/playlist.m3u8"))
	if err != nil {
		return err
	}

	newPLPath := filepath.Join(inputFolder, "playlist_copy.m3u8")

	// Note that WriteFile truncates the file if it exists
	err = os.WriteFile(newPLPath, input, 0666)
	if err != nil {
		return err
	}

	// If we do not do this them FFMPEG just waits for new data. Not what we want.
	f, err := os.OpenFile(newPLPath, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0666)
	if err != nil {
		return err
	}

	defer f.Close()

	_, err = f.WriteString("\n#EXT-X-ENDLIST")
	if err != nil {
		return err
	}

	// FFMPEG mux file
	ffmpegCmd := exec.Command("ffmpeg",
		"-i", newPLPath,
		"-c", "copy",
		"-movflags", "+faststart",
		"-bsf:a", "aac_adtstoasc",
		"-y", outputFile,
	)

	return ffmpegCmd.Run()
}
