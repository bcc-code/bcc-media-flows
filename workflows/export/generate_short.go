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
	"github.com/davecgh/go-spew/spew"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

type GenerateShortResult struct {
	VideoFile *paths.Path
	Keyframes []activities.Keyframe
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

	// Submit the job
	submitJobParams := activities.SubmitShortJobInput{
		URL:        params.ShortServiceURL,
		InputPath:  clipResult.VideoFile.Local(),
		OutputPath: outputDir.Local(),
		Model:      "n",
		Debug:      true,
	}

	var jobResult *activities.SubmitShortJobResult
	err = workflow.ExecuteActivity(ctx, activities.UtilActivities{}.SubmitShortJob, submitJobParams).Get(ctx, &jobResult)
	if err != nil {
		logger.Error("Failed to submit job: " + err.Error())
		return nil, err
	}

	logger.Info("Job submitted with ID: " + jobResult.JobID)

	// Poll for completion
	checkStatusParams := activities.CheckJobStatusInput{
		URL:   params.ShortServiceURL,
		JobID: jobResult.JobID,
	}

	for {
		var statusResult *activities.GenerateShortRequestResult
		err = workflow.ExecuteActivity(ctx, activities.UtilActivities{}.CheckJobStatus, checkStatusParams).Get(ctx, &statusResult)
		if err != nil {
			logger.Error("Failed to check job status: " + err.Error())
			return nil, err
		}

		if statusResult.Status == "completed" {
			logger.Info("Job completed successfully")
			return &GenerateShortResult{
				VideoFile: clipResult.VideoFile,
				Keyframes: statusResult.Keyframes,
			}, nil
		}

		if statusResult.Status == "failed" || statusResult.Status == "error" {
			return nil, fmt.Errorf("job failed with status: %s", statusResult.Status)
		}

		// Wait before polling again
		err = workflow.Sleep(ctx, time.Second*5)
		if err != nil {
			return nil, err
		}
	}
}
