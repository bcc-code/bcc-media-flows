package activities

import (
	"context"
	"fmt"

	"github.com/bcc-code/bcc-media-flows/common"
	"github.com/bcc-code/bcc-media-flows/paths"
	"github.com/bcc-code/bcc-media-flows/services/ffmpeg"
	"github.com/bcc-code/bcc-media-flows/services/telegram"
	"github.com/bcc-code/bcc-media-flows/services/transcode"
	"github.com/bcc-code/bcc-media-flows/utils"
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

func (aa AudioActivities) TranscodeToAudioWav(ctx context.Context, input common.WavAudioInput) (*common.AudioResult, error) {
	log := activity.GetLogger(ctx)
	activity.RecordHeartbeat(ctx, "TranscodeToAudioWav")
	log.Info("Starting TranscodeToAudioAacActivity")

	stopChan, progressCallback := registerProgressCallback(ctx)
	defer close(stopChan)

	return transcode.AudioWav(input, progressCallback)
}

type PrepareTranscriptionResult struct {
	OutputPath paths.Path
	HasAudio   bool
}

func (aa AudioActivities) PrepareForTranscription(ctx context.Context, input common.AudioInput) (*PrepareTranscriptionResult, error) {
	log := activity.GetLogger(ctx)
	activity.RecordHeartbeat(ctx, "TranscodeToAudioWav")
	log.Info("Starting TranscodeToAudioAacActivity")

	stopChan, progressCallback := registerProgressCallback(ctx)
	defer close(stopChan)

	result, err := aa.AnalyzeFile(ctx, AnalyzeFileParams{FilePath: input.Path})
	if err != nil {
		return nil, err
	}

	if !result.HasAudio {
		return &PrepareTranscriptionResult{
			HasAudio: false,
		}, nil
	}

	res, err := transcode.PrepareForTranscriptoion(input, progressCallback)
	if err != nil {
		return nil, err
	}

	return &PrepareTranscriptionResult{
		OutputPath: res.OutputPath,
		HasAudio:   true,
	}, err
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

	// 2400 is the number of samples in 50ms of audio at 48000Hz
	// This seems to be a "standard" offset between youplay and reaper
	samplesToAdd := 2400

	videoSamples := 0

	videoTC, err := ffmpeg.GetTimeCode(input.VideoFile.Local())
	if err != nil {
		log.Warn(err.Error())
		telegram.SendText(telegram.ChatOther, fmt.Sprintf("🟧 Unable to get timecode for `%s`. File imported unadjusted and *WILL* be out of sync with video.", input.AudioFile))
	} else {
		videoSamples, err = utils.TCToSamples(videoTC, 25, 48000)
		if err != nil {
			return nil, err
		}
	}

	audioSamples, err := ffmpeg.GetTimeReference(input.AudioFile.Local())
	if err != nil {
		log.Warn(err.Error())
		telegram.SendText(telegram.ChatOther, fmt.Sprintf("🟧 Unable to get timecode for `%s`. File imported unadjusted and *WILL* be out of sync with video.", input.AudioFile))
	} else if videoSamples > 0 {
		samplesToAdd += audioSamples - videoSamples
	}

	if samplesToAdd < 0 {
		_, err := aa.TrimFile(ctx, TrimInput{
			Output: input.OutputFile,
			Input:  input.AudioFile,
			Start:  float64(-samplesToAdd) / float64(48000),
		})
		return nil, err
	}

	_, err = aa.PrependSilence(ctx, PrependSilenceInput{
		FilePath:   input.AudioFile,
		Output:     input.OutputFile,
		SampleRate: 48000,
		Samples:    samplesToAdd,
	})

	return &common.AudioResult{}, nil
}

func (aa AudioActivities) DetectSilence(ctx context.Context, input common.DetectSilenceInput) (bool, error) {
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
			return nil, fmt.Errorf("channel %d not found in video", channel)
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

// Convert51to4Mono is a special type of conversion that is used to convert a 5.1 audio stream to 4 mono streams (L, R, Lb, Rb) in a video file.
// It is used for the Abekas export workflow, and is not intended to be used in other contexts.
func (aa AudioActivities) Convert51to4Mono(ctx context.Context, input common.AudioInput) (*common.AudioResult, error) {
	log := activity.GetLogger(ctx)
	activity.RecordHeartbeat(ctx, "Convert51to4Mono")
	log.Info("Starting Convert51to4MonoActivity")

	stopChan, progressCallback := registerProgressCallback(ctx)
	defer close(stopChan)

	err := transcode.Convert51to4Mono(input.Path, input.DestinationPath, progressCallback)
	return &common.AudioResult{
		OutputPath: input.DestinationPath,
	}, err
}

type ToneInput struct {
	Frequency       int
	Duration        float64
	SampleRate      int
	TimeCode        string
	DestinationFile paths.Path
}

// GenerateToneFile generates a tone file with the specified frequency, duration, sample rate and time code.
//
// These files are used as pilot tones in the audio playback at Oslofjord.
func (aa AudioActivities) GenerateToneFile(ctx context.Context, input ToneInput) (*common.AudioResult, error) {
	log := activity.GetLogger(ctx)
	activity.RecordHeartbeat(ctx, "GenerateToneFile")
	log.Info("Starting GenerateToneFileActivity")

	err := transcode.GenerateToneFile(input.Frequency, input.Duration, input.SampleRate, input.TimeCode, input.DestinationFile)
	return &common.AudioResult{
		OutputPath: input.DestinationFile,
	}, err
}
