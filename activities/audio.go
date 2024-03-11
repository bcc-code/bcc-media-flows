package activities

import (
	"context"

	"github.com/bcc-code/bcc-media-flows/common"
	"github.com/bcc-code/bcc-media-flows/paths"
	"github.com/bcc-code/bcc-media-flows/services/ffmpeg"
	"github.com/bcc-code/bcc-media-flows/services/transcode"
	"github.com/bcc-code/bcc-media-flows/utils"
	"github.com/go-errors/errors"
	"go.temporal.io/sdk/activity"
)

func TranscodeToAudioAac(ctx context.Context, input common.AudioInput) (*common.AudioResult, error) {
	log := activity.GetLogger(ctx)
	activity.RecordHeartbeat(ctx, "TranscodeToAudioAac")
	log.Info("Starting TranscodeToAudioAacActivity")

	stopChan, progressCallback := registerProgressCallback(ctx)
	defer close(stopChan)

	return transcode.AudioAac(input, progressCallback)
}

func TranscodeToAudioWav(ctx context.Context, input common.AudioInput) (*common.AudioResult, error) {
	log := activity.GetLogger(ctx)
	activity.RecordHeartbeat(ctx, "TranscodeToAudioWav")
	log.Info("Starting TranscodeToAudioAacActivity")

	stopChan, progressCallback := registerProgressCallback(ctx)
	defer close(stopChan)

	return transcode.AudioWav(input, progressCallback)
}

type AdjustAudioToVideoStartInput struct {
	VideoFile  paths.Path
	AudioFile  paths.Path
	OutputFile paths.Path
}

func AdjustAudioToVideoStart(ctx context.Context, input AdjustAudioToVideoStartInput) (*common.AudioResult, error) {
	log := activity.GetLogger(ctx)
	activity.RecordHeartbeat(ctx, "AdjustAudioToVideoStart")
	log.Info("Starting AdjustAudioToVideoStartActivity")

	videoTC, err := ffmpeg.GetTimeCode(input.VideoFile.Local())
	if err != nil {
		return nil, err
	}

	audioSamples, err := ffmpeg.GetTimeReferencce(input.AudioFile.Local())
	if err != nil {
		return nil, err
	}

	videoSamples, err := utils.TCToSamples(videoTC, 25, 48000)
	if err != nil {
		return nil, err
	}
	// 2400 is the number of samples in 50ms of audio at 48000Hz
	// This seems to be a "standard" offset between youplay and reaper
	samplesToAdd := audioSamples - videoSamples + 2400

	if samplesToAdd < 0 {
		return nil, errors.New("Audio starts before video. This is currently not supported")
	}

	_, err = PrependSilence(ctx, PrependSilenceInput{
		FilePath:   input.AudioFile,
		Output:     input.OutputFile,
		SampleRate: 48000,
		Samples:    samplesToAdd,
	})

	return &common.AudioResult{}, nil
}

func DetectSilence(ctx context.Context, input common.AudioInput) (bool, error) {
	log := activity.GetLogger(ctx)
	activity.RecordHeartbeat(ctx, "DetectSilence")
	log.Info("Starting DetectSilenceActivity")

	return transcode.AudioIsSilent(input.Path)
}

type GetVideoOffsetInput struct {
	VideoPath1 paths.Path
	VideoPath2 paths.Path
}

func GetVideoOffset(ctx context.Context, input GetVideoOffsetInput) (int, error) {
	log := activity.GetLogger(ctx)
	activity.RecordHeartbeat(ctx, "GetVideoOffset")
	log.Info("Starting GetVideoOffsetActivity")

	video1TC, err := ffmpeg.GetTimeCode(input.VideoPath1.Local())
	if err != nil {
		return 0, err
	}

	video2TC, err := ffmpeg.GetTimeCode(input.VideoPath2.Local())
	if err != nil {
		return 0, err
	}

	// Video from YouPlay is always 25fps and 48000Hz
	videoSamples1, err := utils.TCToSamples(video1TC, 25, 48000)
	if err != nil {
		return 0, err
	}

	videoSamples2, err := utils.TCToSamples(video2TC, 25, 48000)
	if err != nil {
		return 0, err
	}

	return videoSamples2 - videoSamples1, nil
}

type ExtractAudioInput struct {
	VideoPath       paths.Path
	OutputFolder    paths.Path
	FileNamePattern string
	Channels        []int
}

type ExtractAudioOutput struct {
	AudioFiles []paths.Path
}

// ExtractAudio extracts audio from a video file.
//   - VideoPath: the path to the video file
//   - OutputFolder: the folder where the audio files will be saved
//   - FileNamePattern: the pattern for the audio files. The pattern should contain one %d which will be replaced by the channel number
//   - Channels: the channels to extract. If empty, all channels will be extracted
func ExtractAudio(ctx context.Context, input ExtractAudioInput) (*ExtractAudioOutput, error) {
	log := activity.GetLogger(ctx)
	activity.RecordHeartbeat(ctx, "ExtractAudio")
	log.Info("Starting ExtractAudioActivity")

	panic("Not implemented")

	return nil, nil
}
