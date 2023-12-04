package activities

import (
	"context"
	"fmt"

	"github.com/bcc-code/bccm-flows/common"
	"github.com/bcc-code/bccm-flows/paths"
	"github.com/bcc-code/bccm-flows/services/ffmpeg"
	"github.com/bcc-code/bccm-flows/services/transcode"
	"go.temporal.io/sdk/activity"
)

type EncodeParams struct {
	FilePath   paths.Path
	OutputDir  paths.Path
	Resolution string
	FrameRate  int
	Bitrate    string
	Interlace  bool
}

type EncodeResult struct {
	OutputPath paths.Path
}

func TranscodeToProResActivity(ctx context.Context, input EncodeParams) (*EncodeResult, error) {
	log := activity.GetLogger(ctx)
	activity.RecordHeartbeat(ctx, "TranscodeToProRes")
	log.Info("Starting TranscodeToProResActivity")

	stop, progressCallback := registerProgressCallback(ctx)
	defer close(stop)

	transcodeResult, err := transcode.ProRes(transcode.ProResInput{
		FilePath:   input.FilePath.Local(),
		OutputDir:  input.OutputDir.Local(),
		FrameRate:  input.FrameRate,
		Resolution: input.Resolution,
	}, progressCallback)
	if err != nil {
		fmt.Println(err.Error())
		return nil, err
	}

	return &EncodeResult{
		OutputPath: paths.MustParse(transcodeResult.OutputPath),
	}, nil
}

func TranscodeToH264Activity(ctx context.Context, input EncodeParams) (*EncodeResult, error) {
	log := activity.GetLogger(ctx)
	activity.RecordHeartbeat(ctx, "TranscodeToH264")
	log.Info("Starting TranscodeToH264Activity")

	stop, progressCallback := registerProgressCallback(ctx)
	defer close(stop)

	transcodeResult, err := transcode.H264(transcode.EncodeInput{
		FilePath:   input.FilePath.Local(),
		OutputDir:  input.OutputDir.Local(),
		FrameRate:  input.FrameRate,
		Resolution: input.Resolution,
		Bitrate:    input.Bitrate,
	}, progressCallback)
	if err != nil {
		return nil, err
	}

	return &EncodeResult{
		OutputPath: paths.MustParse(transcodeResult.Path),
	}, nil
}

func TranscodeToXDCAMActivity(ctx context.Context, input EncodeParams) (*EncodeResult, error) {
	log := activity.GetLogger(ctx)
	activity.RecordHeartbeat(ctx, "TranscodeToXDCAM")
	log.Info("Starting TranscodeToXDCAMActivity")

	stop, progressCallback := registerProgressCallback(ctx)
	defer close(stop)

	transcodeResult, err := transcode.XDCAM(transcode.EncodeInput{
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

func TranscodeMergeVideo(ctx context.Context, params common.MergeInput) (*common.MergeResult, error) {
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

func TranscodeMergeAudio(ctx context.Context, params common.MergeInput) (*common.MergeResult, error) {
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

func TranscodeMergeSubtitles(ctx context.Context, params common.MergeInput) (*common.MergeResult, error) {
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

func TranscodeToVideoH264(ctx context.Context, input common.VideoInput) (*common.VideoResult, error) {
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

func TranscodeToAudioMP3(ctx context.Context, input common.AudioInput) (*common.AudioResult, error) {
	log := activity.GetLogger(ctx)
	activity.RecordHeartbeat(ctx, "TranscodeToAudioMP3")
	log.Info("Starting TranscodeToAudioMP3Activity")

	stopChan, progressCallback := registerProgressCallback(ctx)
	defer close(stopChan)

	return transcode.AudioMP3(input, progressCallback)
}

func TranscodeMux(ctx context.Context, input common.MuxInput) (*common.MuxResult, error) {
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

func TranscodePlayoutMux(ctx context.Context, input common.PlayoutMuxInput) (*common.PlayoutMuxResult, error) {
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

func ExecuteFFmpeg(ctx context.Context, input ExecuteFFmpegInput) error {
	log := activity.GetLogger(ctx)
	activity.RecordHeartbeat(ctx, "ExecuteFFmpeg")
	log.Info("Starting ExecuteFFmpeg")

	stopChan, progressCallback := registerProgressCallback(ctx)
	defer close(stopChan)

	_, err := ffmpeg.Do(input.Arguments, ffmpeg.StreamInfo{}, progressCallback)
	if err != nil {
		return err
	}
	return nil
}

type SubtitleBurnInInput struct {
	OutputPath   paths.Path
	VideoFile    paths.Path
	SubtitleFile paths.Path
}

type SubtitleBurnInOutput struct {
	OutputFile paths.Path
}

func SubtitleBurnIn(ctx context.Context, input SubtitleBurnInInput) (*SubtitleBurnInOutput, error) {
	log := activity.GetLogger(ctx)
	activity.RecordHeartbeat(ctx, "SubtitleBurnIn")
	log.Info("Starting SubtitleBurnIn")

	stopChan, progressCallback := registerProgressCallback(ctx)
	defer close(stopChan)

	output, err := transcode.SubtitleBurnIn(input.VideoFile, input.SubtitleFile, input.OutputPath, progressCallback)
	if err != nil {
		return nil, err
	}

	return &SubtitleBurnInOutput{
		OutputFile: *output,
	}, nil
}
