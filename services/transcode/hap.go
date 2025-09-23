package transcode

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bcc-code/bcc-media-flows/services/ffmpeg"
)

type HAPInput struct {
	FilePath  string
	OutputDir string
}

type HAPResult struct {
	OutputPath string
}

func HAP(input HAPInput, progressCallback ffmpeg.ProgressCallback) (*HAPResult, error) {
	info, err := ffmpeg.GetStreamInfo(input.FilePath)
	if err != nil {
		return nil, err
	}

	if !info.HasVideo {
		return nil, fmt.Errorf("input file has no video stream")
	}

	filename := filepath.Base(strings.TrimSuffix(input.FilePath, filepath.Ext(input.FilePath))) + ".mov"
	outputPath := filepath.Join(input.OutputDir, filename)

	var audioFiles []string
	var tempVideoPath string

	// Step 1: Extract all audio tracks as WAV files at once if present
	if info.HasAudio {
		baseFilename := strings.TrimSuffix(filename, filepath.Ext(filename))

		// Build audio extraction command for all tracks at once
		audioParams := []string{
			"-progress", "pipe:1",
			"-hide_banner",
			"-i", input.FilePath,
		}

		// Add mapping and output for each audio track
		for i := 0; i < len(info.AudioStreams); i++ {
			audioFilename := fmt.Sprintf("%s_audio_%d.wav", baseFilename, i)
			audioPath := filepath.Join(input.OutputDir, audioFilename)
			audioFiles = append(audioFiles, audioPath)

			audioParams = append(audioParams,
				"-map", fmt.Sprintf("0:a:%d", i),
				"-c:a", "pcm_s24le",
				audioPath,
			)
		}

		// Add overwrite flag
		audioParams = append(audioParams, "-y")

		_, err = ffmpeg.Do(audioParams, info, progressCallback)
		if err != nil {
			// Clean up any created audio files on error
			for _, af := range audioFiles {
				_ = os.Remove(af) // Ignore cleanup errors
			}
			return nil, fmt.Errorf("failed to extract audio tracks: %w", err)
		}

		// Use temporary video file for intermediate step
		tempVideoPath = filepath.Join(input.OutputDir, fmt.Sprintf("%s_video_only.mov", baseFilename))
	} else {
		// No audio, output directly to final path
		tempVideoPath = outputPath
	}

	// Step 2: Encode video without audio
	videoParams := []string{
		"-progress", "pipe:1",
		"-hide_banner",
		"-i", input.FilePath,
		"-c:v", "hap",
		"-format", "hap_q",
		"-r", "50",
		"-map", "0:v:0",
		"-an", // Explicitly exclude audio
		"-y", tempVideoPath,
	}

	_, err = ffmpeg.Do(videoParams, info, progressCallback)
	if err != nil {
		// Clean up audio files on error
		for _, af := range audioFiles {
			_ = os.Remove(af) // Ignore cleanup errors
		}
		if tempVideoPath != outputPath {
			_ = os.Remove(tempVideoPath) // Ignore cleanup errors
		}
		return nil, fmt.Errorf("failed to encode HAP video: %w", err)
	}

	// Step 3: Mux audio tracks back if any were extracted
	if len(audioFiles) > 0 {
		muxParams := []string{
			"-progress", "pipe:1",
			"-hide_banner",
			"-i", tempVideoPath,
		}

		// Add all audio files as inputs
		for _, audioFile := range audioFiles {
			muxParams = append(muxParams, "-i", audioFile)
		}

		// Map video stream first
		muxParams = append(muxParams, "-map", "0:v:0")

		// Map and encode audio streams
		for i := range audioFiles {
			muxParams = append(muxParams, "-map", fmt.Sprintf("%d:a:0", i+1))
		}

		// Set codecs
		muxParams = append(muxParams, "-c:v", "copy")
		muxParams = append(muxParams, "-c:a", "pcm_s24le")

		// Output to final path
		muxParams = append(muxParams, "-y", outputPath)

		_, err = ffmpeg.Do(muxParams, info, progressCallback)
		if err != nil {
			// Clean up temporary files on error
			_ = os.Remove(tempVideoPath) // Ignore cleanup errors
			for _, af := range audioFiles {
				_ = os.Remove(af) // Ignore cleanup errors
			}
			return nil, fmt.Errorf("failed to mux audio with HAP video: %w", err)
		}

		// Clean up temporary files after successful muxing
		_ = os.Remove(tempVideoPath) // Ignore cleanup errors
		for _, af := range audioFiles {
			_ = os.Remove(af) // Ignore cleanup errors
		}
	}

	err = os.Chmod(outputPath, os.ModePerm)
	if err != nil {
		return nil, err
	}

	return &HAPResult{
		OutputPath: outputPath,
	}, nil
}
