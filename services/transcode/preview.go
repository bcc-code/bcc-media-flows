package transcode

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	bccmflows "github.com/bcc-code/bcc-media-flows"
	"github.com/bcc-code/bcc-media-flows/environment"

	"github.com/bcc-code/bcc-media-flows/services/ffmpeg"
)

type PreviewInput struct {
	FilePath  string
	OutputDir string
}

type PreviewResult struct {
	LowResolutionPath string
	AudioOnly         bool
}

type AudioPreviewResult struct {
	AudioTracks map[string]string
}

var previewWatermarkPath = environment.GetIsilonPrefix() + "/system/graphics/LOGO_BTV_Preview_960-540.mov"

type audioPreviewData struct {
	FFMPEGParams []string
	LanguageMap  map[string]string
}

func prepareAudioPreview(isMU1, isMU2 bool, fileInfo *ffmpeg.FFProbeResult, inputFile, outputDir string) (*audioPreviewData, error) {
	audioStreams := []ffmpeg.FFProbeStream{}
	for _, stream := range fileInfo.Streams {
		if stream.CodecType == "audio" {
			audioStreams = append(audioStreams, stream)
		}
	}

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
			return nil, fmt.Errorf("unknown format of audio channels. Not generating preview")
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

	info, err := ffmpeg.ProbeFile(input.FilePath)
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
			"-i", previewWatermarkPath,
			"-filter_complex", "sws_flags=bicubic;[0:v]split=1[VIDEO-main-.mp4];[VIDEO-main-.mp4]scale=-2:540,null[temp];[temp][1:v]overlay=0:0:eof_action=repeat[VIDEO-.mp4]",
			"-map", "[VIDEO-.mp4]",
			"-c:v", encoder,
		)
	} else if hasVideo {
		params = append(params,
			"-ac", "2",
			"-ss", "0.0",
			"-i", input.FilePath,
			"-ss", "0.0",
			"-i", previewWatermarkPath,
			"-filter_complex", "sws_flags=bicubic;[0:v]split=1[VIDEO-main-.mp4];[VIDEO-main-.mp4]scale=-2:540,null[temp];[temp][1:v]overlay=0:0:eof_action=repeat[VIDEO-.mp4];[0:a]pan=stereo|c0=c0|c1=c1[AUDIO-.mp4-0]",
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

	_, err = ffmpeg.Do(params, ffmpeg.ProbeResultToInfo(info), progressCallback)
	if err != nil {
		return nil, err
	}

	err = os.Chmod(outputPath, os.ModePerm)
	if err != nil {
		return nil, err
	}

	return &PreviewResult{
		LowResolutionPath: outputPath,
		AudioOnly:         !hasVideo && hasAudio,
	}, nil
}
