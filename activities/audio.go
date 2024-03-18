package activities

import (
	"context"
	"fmt"

	"github.com/bcc-code/bcc-media-flows/common"
	"github.com/bcc-code/bcc-media-flows/paths"
	"github.com/bcc-code/bcc-media-flows/services/ffmpeg"
	"github.com/bcc-code/bcc-media-flows/services/transcode"
	"github.com/bcc-code/bcc-media-flows/utils"
	"github.com/go-errors/errors"
	"github.com/samber/lo"
	"go.temporal.io/sdk/activity"
)

func (aa AudioActivities) TranscodeToAudioAac(ctx context.Context, input common.AudioInput) (*common.AudioResult, error) {
	log := activity.GetLogger(ctx)
	activity.RecordHeartbeat(ctx, "TranscodeToAudioAac")
	log.Info("Starting TranscodeToAudioAacActivity")

	stopChan, progressCallback := registerProgressCallback(ctx)
	defer close(stopChan)

	return transcode.AudioAac(input, progressCallback)
}

func (aa AudioActivities) TranscodeToAudioWav(ctx context.Context, input common.AudioInput) (*common.AudioResult, error) {
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

func (aa AudioActivities) AdjustAudioToVideoStart(ctx context.Context, input AdjustAudioToVideoStartInput) (*common.AudioResult, error) {
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

	_, err = aa.PrependSilence(ctx, PrependSilenceInput{
		FilePath:   input.AudioFile,
		Output:     input.OutputFile,
		SampleRate: 48000,
		Samples:    samplesToAdd,
	})

	return &common.AudioResult{}, nil
}

func (aa AudioActivities) DetectSilence(ctx context.Context, input common.AudioInput) (bool, error) {
	log := activity.GetLogger(ctx)
	activity.RecordHeartbeat(ctx, "DetectSilence")
	log.Info("Starting DetectSilenceActivity")

	return transcode.AudioIsSilent(input.Path)
}

type ExtractAudioInput struct {
	VideoPath       paths.Path
	OutputFolder    paths.Path
	FileNamePattern string
	Channels        []int
}

type ExtractAudioOutput struct {
	AudioFiles map[int]paths.Path
}

// ExtractAudio extracts audio from a video file.
//   - VideoPath: the path to the video file
//   - OutputFolder: the folder where the audio files will be saved
//   - FileNamePattern: the pattern for the audio files. The pattern should contain one %d which will be replaced by the channel number
//   - Channels: the channels to extract. If empty, all channels will be extracted
func (aa AudioActivities) ExtractAudio(ctx context.Context, input ExtractAudioInput) (*ExtractAudioOutput, error) {
	log := activity.GetLogger(ctx)
	activity.RecordHeartbeat(ctx, "ExtractAudio")
	log.Info("Starting ExtractAudioActivity")

	availableChannels := map[int]ffmpeg.FFProbeStream{}

	analyzed, err := aa.AnalyzeFile(ctx, AnalyzeFileParams{
		FilePath: input.VideoPath,
	})

	if err != nil {
		return nil, err
	}

	for _, stream := range analyzed.AudioStreams {
		availableChannels[stream.Index] = stream
	}

	if len(input.Channels) == 0 {
		input.Channels = lo.Keys(availableChannels)
	}

	extractedChannels := map[int]paths.Path{}

	for _, channel := range input.Channels {
		if _, ok := availableChannels[channel]; !ok {
			return nil, errors.Errorf("Channel %d not found in video", channel)
		}

		extractedChannels[channel] = input.OutputFolder.Append(fmt.Sprintf(input.FileNamePattern, channel))
	}

	stopChan, progressCallback := registerProgressCallback(ctx)
	defer close(stopChan)

	_, err = transcode.ExtractAudioChannels(input.VideoPath, extractedChannels, progressCallback)

	return &ExtractAudioOutput{
		AudioFiles: extractedChannels,
	}, err

}
