package export

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/bcc-code/bcc-media-flows/activities"
	"github.com/bcc-code/bcc-media-flows/paths"
	"github.com/bcc-code/bcc-media-flows/services/vidispine"
	wfutils "github.com/bcc-code/bcc-media-flows/utils/workflows"
	miscworkflows "github.com/bcc-code/bcc-media-flows/workflows/misc"
	"github.com/davecgh/go-spew/spew"
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

	spew.Dump(params, "SUS")

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

	spew.Dump(mergeExportDataParams)

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
		"-c:a", "copy",
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
	if len(keyframes) == 0 {
		return "489" // fallback value
	}

	if len(keyframes) == 1 {
		value := getParamValue(keyframes[0], param)
		return fmt.Sprintf("%d", value)
	}

	var conditions []string

	// process keyframes in reverse order (for nested if structure)
	for i := len(keyframes) - 1; i >= 1; i-- {
		currentKf := keyframes[i]

		if currentKf.JumpCut {
			// jump cut
			targetValue := getParamValue(currentKf, param)
			condition := fmt.Sprintf("if(gte(t,%.3f),%d,", currentKf.StartTimestamp, targetValue)
			conditions = append(conditions, condition)
		} else {
			// smooth pan
			prevValue := getParamValue(keyframes[i-1], param)
			targetValue := getParamValue(currentKf, param)

			panDuration := 1.0 // You can make this configurable
			endTime := currentKf.StartTimestamp + panDuration

			// smooth transition expression
			smoothExpr := fmt.Sprintf("if(lte(t,%.3f),%d+(%d-%d)*(t-%.3f)/%.3f,%d)",
				endTime,
				prevValue, targetValue, prevValue,
				currentKf.StartTimestamp, panDuration,
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
