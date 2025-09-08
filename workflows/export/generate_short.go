package export

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	vsactivity "github.com/bcc-code/bcc-media-flows/activities/vidispine"

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
	AudioFiles     map[string]paths.Path
	SubtitleFiles  map[string]paths.Path
	ShortVideoFile *paths.Path
	Keyframes      []activities.Keyframe
}

type GenerateShortDataParams struct {
	VXID          string  `json:"VXID"`
	OutputDirPath string  `json:"OutputDir"`
	InSeconds     float64 `json:"InSeconds"`
	OutSeconds    float64 `json:"OutSeconds"`
	ModelSize     string  `json:"ModelSize"`
	DebugMode     bool    `json:"DebugMode"`
}

var badChars = regexp.MustCompile(`[^a-zA-Z0-9-]`)

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

	outputPath, err := paths.Parse(params.OutputDirPath)
	if err != nil {
		return nil, err
	}

	exportData, err := wfutils.Execute(ctx, activities.Vidispine.GetExportDataActivity, vsactivity.GetExportDataParams{
		VXID:        params.VXID,
		Languages:   []string{"nor", "deu", "eng"},
		AudioSource: vidispine.ExportAudioSourceEmbedded.Value,
		Subclip:     "",
		SubsAllowAI: true,
	}).Result(ctx)

	if err != nil {
		return nil, err
	}

	if len(exportData.Clips) != 1 {
		return nil, fmt.Errorf("only one clip supported, got %d", len(exportData.Clips))
	}

	// transcriptFile := exportData.Clips[0].JSONTranscriptFile

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

	titleWithShort := badChars.ReplaceAllString(exportData.Title, "_") + "_short"

	clip := exportData.Clips[0]
	clip.InSeconds = params.InSeconds
	clip.OutSeconds = params.OutSeconds

	mergeExportDataParams := MergeExportDataParams{
		ExportData:       exportData,
		TempDir:          tempFolder,
		SubtitlesDir:     subtitlesOutputDir,
		MakeVideo:        true,
		MakeAudio:        true,
		MakeSubtitles:    true,
		MakeTranscript:   true,
		Languages:        []string{"nor", "deu", "eng"},
		OriginalLanguage: exportData.OriginalLanguage,
	}

	var clipResult MergeExportDataResult
	err = workflow.ExecuteChildWorkflow(ctx, MergeExportData, mergeExportDataParams).Get(ctx, &clipResult)
	if err != nil {
		return nil, err
	}

	sceneResult, err := wfutils.Execute(ctx,
		activities.Video.FFmpegGetSceneChanges,
		clipResult.VideoFile,
	).Result(ctx)
	if err != nil {
		logger.Error("Scene-detect FFmpeg failed: " + err.Error())
		return nil, err
	}

	submitJobParams := activities.SubmitShortJobInput{
		InputPath:    clipResult.VideoFile.Linux(),
		OutputPath:   tempFolder.Linux(),
		Model:        params.ModelSize,
		Debug:        params.DebugMode,
		SceneChanges: sceneResult,
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

	shortVideoPath := outputPath.Append(titleWithShort + "_cropped.mov")

	var subtitlePaths *paths.Path
	if s, ok := clipResult.SubtitleFiles["no"]; ok {
		subtitlePaths = &s
	} else if s, ok := clipResult.SubtitleFiles["und"]; ok {
		subtitlePaths = &s
	}

	var cropRes activities.CropShortResult
	err = wfutils.Execute(ctx,
		activities.Util.CropShortActivity,
		activities.CropShortInput{
			InputVideoPath:  *clipResult.VideoFile,
			OutputVideoPath: shortVideoPath,
			SubtitlePath:    subtitlePaths,
			KeyFrames:       keyframes,
			InSeconds:       params.InSeconds,
			OutSeconds:      params.OutSeconds,
			SceneChanges:    sceneResult,
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
		AudioFiles:     clipResult.AudioFiles,
		SubtitleFiles:  clipResult.SubtitleFiles,
	}, nil
}
