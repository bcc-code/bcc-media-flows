package export

import (
	"fmt"
	vsactivity "github.com/bcc-code/bcc-media-flows/activities/vidispine"
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
	VXID          string  `json:"VXID"`
	OutputDirPath string  `json:"OutputDir"`
	InSeconds     float64 `json:"InSeconds"`
	OutSeconds    float64 `json:"OutSeconds"`
}

func validationError(msg string) error {
	return temporal.NewApplicationError(msg, "ValidationError")
}

func GenerateShort(ctx workflow.Context, params GenerateShortDataParams) (*GenerateShortResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting GenerateShort")

	if strings.TrimSpace(params.VXID) == "" {
		return nil, validationError("VXID is empty")
	}
	if strings.TrimSpace(params.OutputDirPath) == "" {
		return nil, validationError("OutputDirPath is empty")
	}
	if params.InSeconds < 0 {
		return nil, validationError("InSeconds must be >= 0")
	}
	if params.OutSeconds < 0 {
		return nil, validationError("OutSeconds must be >= 0")
	}
	if params.InSeconds >= params.OutSeconds {
		return nil, validationError("InSeconds must be < OutSeconds")
	}

	originalFile, err := wfutils.Execute(ctx, activities.Vidispine.GetFileFromVXActivity, vsactivity.GetFileFromVXParams{
		VXID: params.VXID,
		Tags: []string{"original"},
	}).Result(ctx)

	if err != nil {
		return nil, err
	}

	activityOptions := wfutils.GetDefaultActivityOptions()
	ctx = workflow.WithActivityOptions(ctx, activityOptions)

	tempFolder, err := wfutils.GetWorkflowTempFolder(ctx)
	if err != nil {
		return nil, err
	}

	subtitlesOutputDir := tempFolder.Append("subtitles")
	err = wfutils.CreateFolder(ctx, subtitlesOutputDir)
	if err != nil {
		return nil, err
	}

	originalFileName := originalFile.FilePath
	fileNameWithoutExt := originalFileName.BaseNoExt()
	titleWithShort := fileNameWithoutExt + "_short"

	clip := vidispine.Clip{
		VideoFile:          originalFileName.Linux(),
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
		OriginalLanguage:    "no",
		TranscribedLanguage: "",
	}

	mergeExportDataParams := MergeExportDataParams{
		ExportData:       &data,
		TempDir:          tempFolder,
		SubtitlesDir:     subtitlesOutputDir,
		MakeVideo:        true,
		MakeAudio:        false,
		MakeSubtitles:    true,
		MakeTranscript:   true,
		Languages:        []string{"no"},
		OriginalLanguage: data.OriginalLanguage,
	}

	var clipResult MergeExportDataResult
	err = workflow.ExecuteChildWorkflow(ctx, MergeExportData, mergeExportDataParams).Get(ctx, &clipResult)
	if err != nil {
		return nil, err
	}

	submitJobParams := activities.SubmitShortJobInput{
		InputPath:  clipResult.VideoFile.Linux(),
		OutputPath: tempFolder.Linux(),
		Model:      "n",
		Debug:      true,
	}

	jobResult, err := wfutils.Execute(ctx, activities.Util.SubmitShortJobActivity, submitJobParams).Result(ctx)
	if err != nil {
		logger.Error("Failed to submit job: " + err.Error())
		return nil, err
	}

	logger.Info("Job submitted with ID: " + jobResult.JobID)

	checkStatusParams := activities.CheckJobStatusInput{
		JobID: jobResult.JobID,
	}

	var keyframes []activities.Keyframe
	for {
		statusResult, err := wfutils.Execute(ctx, activities.Util.CheckJobStatusActivity, checkStatusParams).Result(ctx)
		if err != nil {
			logger.Error("Failed to check job status: " + err.Error())
			return nil, err
		}

		if statusResult.Status == "completed" {
			logger.Info("Job completed successfully")
			keyframes = statusResult.Keyframes
			break
		}

		if statusResult.Status != "in_progress" {
			return nil, fmt.Errorf("job failed with status: %s", statusResult.Status)
		}

		err = workflow.Sleep(ctx, time.Second*5)
		if err != nil {
			return nil, err
		}
	}

	shortVideoPath := tempFolder.Append(titleWithShort + "_cropped.mp4")

	var cropRes activities.CropShortResult
	err = wfutils.Execute(ctx,
		activities.Util.CropShortActivity,
		activities.CropShortInput{
			InputVideoPath:  clipResult.VideoFile.Local(),
			AudioVideoPath:  originalFileName.Linux(),
			OutputVideoPath: shortVideoPath.Local(),
			KeyFrames:       keyframes,
			InSeconds:       params.InSeconds,
			OutSeconds:      params.OutSeconds,
		}).Get(ctx, &cropRes)
	if err != nil {
		logger.Error("CropShortActivity failed: " + err.Error())
		return nil, err
	}

	ffmpegParams := miscworkflows.ExecuteFFmpegInput{
		Arguments: cropRes.Arguments,
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
