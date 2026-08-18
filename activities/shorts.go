package activities

import (
	"context"
	"fmt"
	"github.com/bcc-code/bcc-media-flows/environment"
	"regexp"
	"strconv"

	"github.com/go-resty/resty/v2"
	"go.temporal.io/sdk/activity"

	"github.com/bcc-code/bcc-media-flows/internal/httpx"
	"github.com/bcc-code/bcc-media-flows/paths"
	"github.com/bcc-code/bcc-media-flows/services/ffmpeg"
)

func shortServiceClient() *resty.Client {
	return httpx.New(httpx.Config{
		Service: "shorts service",
		BaseURL: environment.Get().ShortsServiceURL,
	})
}

type SubmitShortJobInput struct {
	InputPath    string    `json:"input_path"`
	OutputPath   string    `json:"output_path"`
	Model        string    `json:"model"`
	Debug        bool      `json:"debug"`
	SceneChanges []float64 `json:"scene_changes"`
}

type Square struct {
	X int `json:"x"`
	Y int `json:"y"`
	W int `json:"w"`
	H int `json:"h"`
}

type Keyframe struct {
	EndTimestamp   float64 `json:"end_timestamp"`
	JumpCut        bool    `json:"jump_cut"`
	StartTimestamp float64 `json:"start_timestamp"`
	Square
}

type GenerateShortRequestResult struct {
	Debug     string     `json:"debug"`
	Keyframes []Keyframe `json:"keyframes"`
	Status    string     `json:"status"`
}

type SubmitShortJobResult struct {
	JobID string `json:"job_id"`
}

func (ua UtilActivities) SubmitShortJobActivity(ctx context.Context, params SubmitShortJobInput) (*SubmitShortJobResult, error) {
	log := activity.GetLogger(ctx)
	activity.RecordHeartbeat(ctx, "SubmitShortJob")
	log.Info("Starting SubmitShortJob activity")

	var result SubmitShortJobResult
	_, err := shortServiceClient().R().
		SetContext(ctx).
		SetBody(params).
		SetResult(&result).
		Post("/submit_job")

	if err != nil {
		return nil, err
	}

	if result.JobID == "" {
		return nil, fmt.Errorf("shorts service accepted the job but returned no job id")
	}

	return &result, nil
}

type CheckJobStatusInput struct {
	JobID string `json:"job_id"`
}

func (ua UtilActivities) CheckJobStatusActivity(ctx context.Context, params CheckJobStatusInput) (*GenerateShortRequestResult, error) {
	activity.RecordHeartbeat(ctx, "CheckJobStatus")

	var result GenerateShortRequestResult
	_, err := shortServiceClient().R().
		SetContext(ctx).
		SetResult(&result).
		Get("/job_status/" + params.JobID)

	if err != nil {
		return nil, err
	}

	return &result, nil
}

func (va VideoActivities) FFmpegGetSceneChanges(
	ctx context.Context,
	videoFile *paths.Path,
) ([]float64, error) {

	stopChan, progressCallback := registerProgressCallback(ctx)
	defer close(stopChan)

	sceneDetectArgs := []string{
		"-i", videoFile.Local(),
		"-filter_complex", "select='gt(scene,0.1)',metadata=print:file=-",
		"-f", "null", "-",
	}

	out, err := ffmpeg.Do(sceneDetectArgs, ffmpeg.StreamInfo{}, progressCallback)
	if err != nil {
		return nil, err
	}

	raw := string(out)
	re := regexp.MustCompile(`(?m)pts_time:([\d.]+)`)
	matches := re.FindAllStringSubmatch(raw, -1)

	var changes []float64
	for _, m := range matches {
		if len(m) >= 2 {
			t, _ := strconv.ParseFloat(m[1], 64)
			changes = append(changes, t)
		}
	}
	return changes, nil
}
