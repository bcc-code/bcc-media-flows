package export

import (
	"fmt"
	"math"
	"path/filepath"
	"strings"
	"time"

	"github.com/bcc-code/bcc-media-flows/activities"
	"github.com/bcc-code/bcc-media-flows/paths"
	"github.com/bcc-code/bcc-media-flows/services/vidispine"
	wfutils "github.com/bcc-code/bcc-media-flows/utils/workflows"
	miscworkflows "github.com/bcc-code/bcc-media-flows/workflows/misc"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

type GenerateShortResult struct {
	VideoFile      *paths.Path
	ShortVideoFile *paths.Path
	Keyframes      []activities.Keyframe
}

type GenerateShortDataParams struct {
	VideoFilePath   string  `json:"VideoFile"`
	OutputDirPath   string  `json:"OutputDir"`
	InSeconds       float64 `json:"InSeconds"`
	OutSeconds      float64 `json:"OutSeconds"`
	ShortServiceURL string  `json:"ShortServiceURL"`
}

func GenerateShort(ctx workflow.Context, params GenerateShortDataParams) (*GenerateShortResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting GenerateShort")

	activityOptions := workflow.ActivityOptions{
		StartToCloseTimeout: time.Minute * 10,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, activityOptions)

	outputDir := paths.MustParse(params.OutputDirPath)

	subtitlesOutputDir := outputDir.Append("subtitles")
	err := wfutils.CreateFolder(ctx, subtitlesOutputDir)
	if err != nil {
		return nil, err
	}

	originalFileName := filepath.Base(params.VideoFilePath)
	fileNameWithoutExt := strings.TrimSuffix(originalFileName, filepath.Ext(originalFileName))
	titleWithShort := fileNameWithoutExt + "_short"

	clip := vidispine.Clip{
		VideoFile:          params.VideoFilePath,
		InSeconds:          params.InSeconds,
		OutSeconds:         params.OutSeconds,
		SequenceIn:         0,
		SequenceOut:        params.OutSeconds - params.InSeconds,
		AudioFiles:         nil,
		SubtitleFiles:      nil,
		JSONTranscriptFile: "",
		VXID:               "",
	}

	data := vidispine.ExportData{
		Clips:               []*vidispine.Clip{&clip},
		SafeTitle:           titleWithShort,
		Title:               titleWithShort,
		ImportDate:          nil,
		BmmTitle:            nil,
		BmmTrackID:          nil,
		OriginalLanguage:    "",
		TranscribedLanguage: "",
	}

	mergeExportDataParams := MergeExportDataParams{
		ExportData:       &data,
		TempDir:          outputDir,
		SubtitlesDir:     subtitlesOutputDir,
		MakeVideo:        true,
		MakeAudio:        true,
		MakeSubtitles:    false,
		MakeTranscript:   false,
		Languages:        nil,
		OriginalLanguage: data.OriginalLanguage,
	}

	var clipResult MergeExportDataResult
	err = workflow.ExecuteChildWorkflow(ctx, MergeExportData, mergeExportDataParams).Get(ctx, &clipResult)
	if err != nil {
		return nil, err
	}

	submitJobParams := activities.SubmitShortJobInput{
		URL:        params.ShortServiceURL,
		InputPath:  clipResult.VideoFile.Local(),
		OutputPath: outputDir.Local(),
		Model:      "n",
		Debug:      true,
	}

	var jobResult *activities.SubmitShortJobResult
	err = workflow.ExecuteActivity(ctx, activities.UtilActivities{}.SubmitShortJobActivity, submitJobParams).Get(ctx, &jobResult)
	if err != nil {
		logger.Error("Failed to submit job: " + err.Error())
		return nil, err
	}

	logger.Info("Job submitted with ID: " + jobResult.JobID)

	checkStatusParams := activities.CheckJobStatusInput{
		URL:   params.ShortServiceURL,
		JobID: jobResult.JobID,
	}

	var keyframes []activities.Keyframe
	for {
		var statusResult *activities.GenerateShortRequestResult
		err = workflow.ExecuteActivity(ctx, activities.UtilActivities{}.CheckJobStatusActivity, checkStatusParams).Get(ctx, &statusResult)
		if err != nil {
			logger.Error("Failed to check job status: " + err.Error())
			return nil, err
		}

		if statusResult.Status == "completed" {
			logger.Info("Job completed successfully")
			keyframes = statusResult.Keyframes
			break
		}

		if statusResult.Status == "failed" || statusResult.Status == "error" {
			return nil, fmt.Errorf("job failed with status: %s", statusResult.Status)
		}

		err = workflow.Sleep(ctx, time.Second*5)
		if err != nil {
			return nil, err
		}
	}

	shortVideoPath := outputDir.Append(titleWithShort + "_cropped.mp4")
	cropFilter := buildCropFilter(keyframes)

	ffmpegArgs := []string{
		"-i", clipResult.VideoFile.Local(),
		"-filter_complex", cropFilter,
		"-pix_fmt", "yuv420p",
		"-y",
		shortVideoPath.Local(),
	}

	ffmpegParams := miscworkflows.ExecuteFFmpegInput{
		Arguments: ffmpegArgs,
	}

	err = workflow.ExecuteChildWorkflow(ctx, miscworkflows.ExecuteFFmpeg, ffmpegParams).Get(ctx, nil)
	if err != nil {
		logger.Error("Failed to execute FFmpeg: " + err.Error())
		return nil, err
	}

	return &GenerateShortResult{
		VideoFile:      clipResult.VideoFile,
		ShortVideoFile: &shortVideoPath,
		Keyframes:      keyframes,
	}, nil
}

func buildCropFilter(keyframes []activities.Keyframe) string {
	if len(keyframes) == 0 {
		return "crop=960:540:489:29" // fallback
	}

	if len(keyframes) == 1 {
		kf := keyframes[0]
		return fmt.Sprintf("crop=%d:%d:%d:%d", kf.W, kf.H, kf.X, kf.Y)
	}

	cropW := keyframes[0].W
	cropH := keyframes[0].H

	xExpr := buildSmoothTransitionExpression(keyframes, "X")
	yExpr := buildSmoothTransitionExpression(keyframes, "Y")

	return fmt.Sprintf("crop=%d:%d:x='%s':y='%s'", cropW, cropH, xExpr, yExpr)
}

func buildSmoothTransitionExpression(keyframes []activities.Keyframe, param string) string {
	var conditions []string

	for i := len(keyframes) - 1; i >= 1; i-- {
		currentKf := keyframes[i]

		if currentKf.JumpCut {
			targetValue := getParamValue(currentKf, param)
			condition := fmt.Sprintf("if(gte(t,%.3f),%d,", currentKf.StartTimestamp, targetValue)
			conditions = append(conditions, condition)
		} else {
			prevValue := getParamValue(keyframes[i-1], param)
			targetValue := getParamValue(currentKf, param)

			distance := calculateDistance(keyframes[i-1], currentKf)
			panDuration := calculatePanDuration(distance)
			endTime := currentKf.StartTimestamp + panDuration

			normalizedTime := fmt.Sprintf("(t-%.3f)/%.3f", currentKf.StartTimestamp, panDuration)
			easingFactor := fmt.Sprintf("(1-pow(1-(%s),2))", normalizedTime)

			smoothExpr := fmt.Sprintf("if(lte(t,%.3f),%d+(%d-%d)*%s,%d)",
				endTime,
				prevValue, targetValue, prevValue,
				easingFactor,
				targetValue)

			condition := fmt.Sprintf("if(gte(t,%.3f),%s,", currentKf.StartTimestamp, smoothExpr)
			conditions = append(conditions, condition)
		}
	}

	result := strings.Join(conditions, "")
	firstValue := getParamValue(keyframes[0], param)
	result += fmt.Sprintf("%d", firstValue)
	result += strings.Repeat(")", len(conditions))

	return result
}

func calculateDistance(kf1, kf2 activities.Keyframe) float64 {
	dx := float64(kf2.X - kf1.X)
	dy := float64(kf2.Y - kf1.Y)
	return math.Sqrt(dx*dx + dy*dy)
}

func calculatePanDuration(distance float64) float64 {
	const (
		minDuration = 0.1
		maxDuration = 3.0
		speedFactor = 200.0
	)

	duration := distance / speedFactor
	if duration < minDuration {
		duration = minDuration
	}
	if duration > maxDuration {
		duration = maxDuration
	}

	return duration
}

func getParamValue(kf activities.Keyframe, param string) int {
	switch param {
	case "X":
		return kf.X
	case "Y":
		return kf.Y
	case "W":
		return kf.W
	case "H":
		return kf.H
	default:
		return 0
	}
}
