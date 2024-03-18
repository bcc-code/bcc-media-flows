package activities

import (
	"context"

	"github.com/bcc-code/bcc-media-flows/utils"

	"github.com/bcc-code/bcc-media-flows/paths"
	"github.com/bcc-code/bcc-media-flows/services/ffmpeg"
	"go.temporal.io/sdk/activity"
)

type AnalyzeFileParams struct {
	FilePath paths.Path
}

func (ta AudioActivities) AnalyzeFile(ctx context.Context, input AnalyzeFileParams) (*ffmpeg.StreamInfo, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Starting AnalyzeFileActivity")

	info, err := ffmpeg.GetStreamInfo(input.FilePath.Local())
	if err != nil {
		return nil, err
	}
	return &info, nil
}

type GetVideoOffsetInput struct {
	VideoPath1      paths.Path
	VideoPath2      paths.Path
	VideoFPS        int
	AudioSampleRate int
}

func (va VideoActivities) GetVideoOffset(ctx context.Context, input GetVideoOffsetInput) (int, error) {
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
	videoSamples1, err := utils.TCToSamples(video1TC, input.VideoFPS, input.AudioSampleRate)
	if err != nil {
		return 0, err
	}

	videoSamples2, err := utils.TCToSamples(video2TC, input.VideoFPS, input.AudioSampleRate)
	if err != nil {
		return 0, err
	}

	return videoSamples2 - videoSamples1, nil
}
