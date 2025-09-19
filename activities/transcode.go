package activities

import (
	"context"
	"fmt"

	"github.com/bcc-code/bcc-media-flows/utils"

	"github.com/bcc-code/bcc-media-flows/common"
	"github.com/bcc-code/bcc-media-flows/paths"
	"github.com/bcc-code/bcc-media-flows/services/ffmpeg"
	"github.com/bcc-code/bcc-media-flows/services/transcode"
	"go.temporal.io/sdk/activity"
)

type EncodeParams struct {
	FilePath       paths.Path
	AudioPaths     []paths.Path
	OutputDir      paths.Path
	Resolution     *utils.Resolution
	FrameRate      int
	Bitrate        string
	Interlace      bool
	BurnInSubtitle *paths.Path
	SubtitleStyle  *paths.Path
	Alpha          bool
}

type EncodeResult struct {
	OutputPath paths.Path
}

func (va VideoActivities) TranscodeToProResActivity(ctx context.Context, input EncodeParams) (*EncodeResult, error) {
	log := activity.GetLogger(ctx)
	activity.RecordHeartbeat(ctx, "TranscodeToProRes")
	log.Info("Starting TranscodeToProResActivity")

	stop, progressCallback := registerProgressCallback(ctx)
	defer close(stop)

	audioPaths := []string{}
	for _, audioPath := range input.AudioPaths {
		audioPaths = append(audioPaths, audioPath.Local())
	}

	transcodeResult, err := transcode.ProRes(transcode.ProResInput{
		FilePath:       input.FilePath.Local(),
		AudioPaths:     audioPaths,
		OutputDir:      input.OutputDir.Local(),
		FrameRate:      input.FrameRate,
		Resolution:     input.Resolution,
		Use4444:        input.Alpha,
		BurnInSubtitle: input.BurnInSubtitle,
		SubtitleStyle:  input.SubtitleStyle,
	}, progressCallback)
	if err != nil {
		fmt.Println(err.Error())
		return nil, err
	}

	return &EncodeResult{
		OutputPath: paths.MustParse(transcodeResult.OutputPath),
	}, nil
}

func (va VideoActivities) TranscodeToHyperdeckProResActivity(ctx context.Context, input EncodeParams) (*EncodeResult, error) {
	log := activity.GetLogger(ctx)
	activity.RecordHeartbeat(ctx, "TranscodeToHyperdeckProResActivity")
	log.Info("Starting TranscodeToHyperdeckProResActivity")

	stop, progressCallback := registerProgressCallback(ctx)
	defer close(stop)

	audioPaths := []string{}
	for _, audioPath := range input.AudioPaths {
		audioPaths = append(audioPaths, audioPath.Local())
	}

	transcodeResult, err := transcode.ProRes(transcode.ProResInput{
		FilePath:       input.FilePath.Local(),
		AudioPaths:     audioPaths,
		OutputDir:      input.OutputDir.Local(),
		FrameRate:      input.FrameRate,
		Resolution:     input.Resolution,
		ForHyperdeck:   true,
		Use4444:        input.Alpha,
		BurnInSubtitle: input.BurnInSubtitle,
		SubtitleStyle:  input.SubtitleStyle,
	}, progressCallback)
	if err != nil {
		fmt.Println(err.Error())
		return nil, err
	}

	return &EncodeResult{
		OutputPath: paths.MustParse(transcodeResult.OutputPath),
	}, nil
}

func (va VideoActivities) TranscodeToAVCIntraActivity(ctx context.Context, input EncodeParams) (*EncodeResult, error) {
	log := activity.GetLogger(ctx)
	activity.RecordHeartbeat(ctx, "TranscodeToAVCIntra")
	log.Info("Starting TranscodeToAVCIntraActivity")

	stop, progressCallback := registerProgressCallback(ctx)
	defer close(stop)

	transcodeResult, err := transcode.AvcIntra(transcode.AVCIntraEncodeInput{
		FilePath:       input.FilePath.Local(),
		OutputDir:      input.OutputDir.Local(),
		FrameRate:      input.FrameRate,
		Resolution:     input.Resolution,
		Interlace:      input.Interlace,
		BurnInSubtitle: input.BurnInSubtitle,
		SubtitleStyle:  input.SubtitleStyle,
	}, progressCallback)
	if err != nil {
		return nil, err
	}

	return &EncodeResult{
		OutputPath: paths.MustParse(transcodeResult.Path),
	}, nil
}

func (va VideoActivities) TranscodeToH264Activity(ctx context.Context, input EncodeParams) (*EncodeResult, error) {
	log := activity.GetLogger(ctx)
	activity.RecordHeartbeat(ctx, "TranscodeToH264")
	log.Info("Starting TranscodeToH264Activity")

	stop, progressCallback := registerProgressCallback(ctx)
	defer close(stop)

	transcodeResult, err := transcode.H264(transcode.H264EncodeInput{
		FilePath:       input.FilePath.Local(),
		OutputDir:      input.OutputDir.Local(),
		FrameRate:      input.FrameRate,
		Resolution:     input.Resolution,
		Bitrate:        input.Bitrate,
		Interlace:      input.Interlace,
		BurnInSubtitle: input.BurnInSubtitle,
	}, progressCallback)
	if err != nil {
		return nil, err
	}

	return &EncodeResult{
		OutputPath: paths.MustParse(transcodeResult.Path),
	}, nil
}

func (va VideoActivities) TranscodeToXDCAMActivity(ctx context.Context, input EncodeParams) (*EncodeResult, error) {
	log := activity.GetLogger(ctx)
	activity.RecordHeartbeat(ctx, "TranscodeToXDCAM")
	log.Info("Starting TranscodeToXDCAMActivity")

	stop, progressCallback := registerProgressCallback(ctx)
	defer close(stop)

	transcodeResult, err := transcode.XDCAM(transcode.XDCAMEncodeInput{
		FilePath:   input.FilePath.Local(),
		OutputDir:  input.OutputDir.Local(),
		FrameRate:  input.FrameRate,
		Resolution: input.Resolution,
		Bitrate:    input.Bitrate,
		Interlace:  input.Interlace,
	}, progressCallback)
	if err != nil {
		return nil, err
	}

	return &EncodeResult{
		OutputPath: paths.MustParse(transcodeResult.Path),
	}, nil
}

func (va VideoActivities) TranscodeMergeVideo(ctx context.Context, params common.MergeInput) (*common.MergeResult, error) {
	log := activity.GetLogger(ctx)
	activity.RecordHeartbeat(ctx, "TranscodeMergeVideo")
	log.Info("Starting TranscodeMergeVideoActivity")

	stopChan, progressCallback := registerProgressCallback(ctx)
	defer close(stopChan)

	result, err := transcode.MergeVideo(params, progressCallback)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (aa AudioActivities) TranscodeMergeAudio(ctx context.Context, params common.MergeInput) (*common.MergeResult, error) {
	log := activity.GetLogger(ctx)
	activity.RecordHeartbeat(ctx, "TranscodeMergeAudio")
	log.Info("Starting TranscodeMergeAudioActivity")

	stopChan, progressCallback := registerProgressCallback(ctx)
	defer close(stopChan)

	result, err := transcode.MergeAudio(params, progressCallback)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (aa AudioActivities) MergeSubtitlesByOffset(ctx context.Context, params common.MergeInput) (*common.MergeResult, error) {
	log := activity.GetLogger(ctx)
	activity.RecordHeartbeat(ctx, "MergeSubtitlesByOffset")
	log.Info("Starting MergeSubtitlesByOffsetActivity")

	// No easy way of reporting progress, so this just triggers heartbeats
	stopChan, progressCallback := registerProgressCallback(ctx)
	defer close(stopChan)

	result, err := transcode.MergeSubtitlesByOffset(params, progressCallback)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (va VideoActivities) TranscodeMergeSubtitles(ctx context.Context, params common.MergeInput) (*common.MergeResult, error) {
	log := activity.GetLogger(ctx)
	activity.RecordHeartbeat(ctx, "TranscodeMergeSubtitles")
	log.Info("Starting TranscodeMergeSubtitlesActivity")

	// No easy way of reporting progress, so this just triggers heartbeats
	stopChan, progressCallback := registerProgressCallback(ctx)
	defer close(stopChan)

	result, err := transcode.MergeSubtitles(params, progressCallback)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (va VideoActivities) TranscodeToVideoH264(ctx context.Context, input common.VideoInput) (*common.VideoResult, error) {
	log := activity.GetLogger(ctx)
	activity.RecordHeartbeat(ctx, "TranscodeToVideoH264")
	log.Info("Starting TranscodeToVideoH264Activity")

	stopChan, progressCallback := registerProgressCallback(ctx)
	defer close(stopChan)

	result, err := transcode.VideoH264(input, progressCallback)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (aa AudioActivities) TranscodeToAudioMP3(ctx context.Context, input common.AudioInput) (*common.AudioResult, error) {
	log := activity.GetLogger(ctx)
	activity.RecordHeartbeat(ctx, "TranscodeToAudioMP3")
	log.Info("Starting TranscodeToAudioMP3Activity")

	stopChan, progressCallback := registerProgressCallback(ctx)
	defer close(stopChan)

	return transcode.AudioMP3(input, progressCallback)
}

func (va VideoActivities) TranscodeMuxToSimpleMXF(ctx context.Context, input common.SimpleMuxInput) (*common.MuxResult, error) {
	log := activity.GetLogger(ctx)
	activity.RecordHeartbeat(ctx, "TranscodeMuxToSimpleMXF")
	log.Info("Starting TranscodeMuxToSimpleMXFActivity")

	stopChan, progressCallback := registerProgressCallback(ctx)
	defer close(stopChan)

	result, err := transcode.MuxToSimpleMXF(input, progressCallback)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (aa AudioActivities) TranscodeMux(ctx context.Context, input common.MuxInput) (*common.MuxResult, error) {
	log := activity.GetLogger(ctx)
	activity.RecordHeartbeat(ctx, "TranscodeMux")
	log.Info("Starting TranscodeMuxActivity")

	stopChan, progressCallback := registerProgressCallback(ctx)
	defer close(stopChan)

	result, err := transcode.Mux(input, progressCallback)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (va VideoActivities) TranscodePlayoutMux(ctx context.Context, input common.PlayoutMuxInput) (*common.PlayoutMuxResult, error) {
	log := activity.GetLogger(ctx)
	activity.RecordHeartbeat(ctx, "TranscodePlayoutMux")
	log.Info("Starting TranscodePlayoutMux")

	stopChan, progressCallback := registerProgressCallback(ctx)
	defer close(stopChan)

	result, err := transcode.PlayoutMux(input, progressCallback)
	if err != nil {
		return nil, err
	}
	return result, nil
}

type ExecuteFFmpegInput struct {
	Arguments []string
}

func (va VideoActivities) ExecuteFFmpeg(ctx context.Context, input ExecuteFFmpegInput) (any, error) {
	log := activity.GetLogger(ctx)
	activity.RecordHeartbeat(ctx, "ExecuteFFmpeg")
	log.Info("Starting ExecuteFFmpeg")

	stopChan, progressCallback := registerProgressCallback(ctx)
	defer close(stopChan)

	_, err := ffmpeg.Do(input.Arguments, ffmpeg.StreamInfo{}, progressCallback)
	return nil, err
}

type SplitAudioChannelsInput struct {
	FilePath  paths.Path
	OutputDir paths.Path
}

func (aa AudioActivities) SplitAudioChannels(ctx context.Context, input SplitAudioChannelsInput) (paths.Files, error) {
	log := activity.GetLogger(ctx)
	activity.RecordHeartbeat(ctx, "SplitAudioChannels")
	log.Info("Starting SplitAudioChannels")

	stopChan, progressCallback := registerProgressCallback(ctx)
	defer close(stopChan)

	result, err := transcode.SplitAudioChannels(input.FilePath, input.OutputDir, progressCallback)
	if err != nil {
		return nil, err
	}
	return result, nil
}

type MultitrackMuxInput struct {
	Files     paths.Files
	OutputDir paths.Path
}

type MultitrackMuxResult struct {
	OutputPath paths.Path
}

func (va VideoActivities) MultitrackMux(ctx context.Context, input MultitrackMuxInput) (*MultitrackMuxResult, error) {
	log := activity.GetLogger(ctx)
	activity.RecordHeartbeat(ctx, "MultitrackMux")
	log.Info("Starting MultitrackMux")

	stopChan, progressCallback := registerProgressCallback(ctx)
	defer close(stopChan)

	result, err := transcode.MultitrackMux(input.Files, input.OutputDir, progressCallback)
	if err != nil {
		return nil, err
	}
	return &MultitrackMuxResult{
		OutputPath: *result,
	}, nil
}

type PrependSilenceInput struct {
	FilePath   paths.Path
	Output     paths.Path
	Samples    int
	SampleRate int
}

type PrependSilenceOutput struct {
	OutputPath paths.Path
}

func (aa AudioActivities) PrependSilence(ctx context.Context, input PrependSilenceInput) (*PrependSilenceOutput, error) {
	log := activity.GetLogger(ctx)
	activity.RecordHeartbeat(ctx, "PrependSilence")
	log.Info("Starting PrependSilence")

	stopChan, progressCallback := registerProgressCallback(ctx)
	defer close(stopChan)

	length := float64(input.Samples) / float64(input.SampleRate)
	result, err := transcode.PrependSilence(input.FilePath, input.Output, length, input.SampleRate, progressCallback)
	if err != nil {
		return nil, err
	}
	return &PrependSilenceOutput{
		OutputPath: *result,
	}, nil
}

type TrimInput struct {
	Input    paths.Path
	Output   paths.Path
	Start    float64
	End      float64
	Duration float64
}

type TrimResult struct {
	OutputPath paths.Path
}

func (aa AudioActivities) TrimFile(ctx context.Context, input TrimInput) (*TrimResult, error) {
	log := activity.GetLogger(ctx)
	activity.RecordHeartbeat(ctx, "TrimFile")
	log.Info("Starting TrimFile")

	if input.Duration != 0 {
		input.End = input.Start + input.Duration
	}

	stopChan, progressCallback := registerProgressCallback(ctx)
	defer close(stopChan)

	err := transcode.TrimFile(
		input.Input,
		input.Output,
		input.Start,
		input.End,
		progressCallback,
	)

	if err != nil {
		return nil, err
	}

	return &TrimResult{
		OutputPath: input.Output,
	}, nil
}

type HAPInput struct {
	FilePath  paths.Path
	OutputDir paths.Path
}

type HAPResult struct {
	OutputPath paths.Path
}

func (va VideoActivities) TranscodeToHAPActivity(ctx context.Context, input HAPInput) (*HAPResult, error) {
	log := activity.GetLogger(ctx)
	activity.RecordHeartbeat(ctx, "TranscodeToHAP")
	log.Info("Starting TranscodeToHAPActivity")

	stop, progressCallback := registerProgressCallback(ctx)
	defer close(stop)

	transcodeResult, err := transcode.HAP(transcode.HAPInput{
		FilePath:  input.FilePath.Local(),
		OutputDir: input.OutputDir.Local(),
	}, progressCallback)
	if err != nil {
		return nil, err
	}

	return &HAPResult{
		OutputPath: paths.MustParse(transcodeResult.OutputPath),
	}, nil
}
