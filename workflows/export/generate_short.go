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

	// Set before the first activity, so every activity in the workflow runs
	// under the same options.
	ctx = workflow.WithActivityOptions(ctx, wfutils.GetDefaultActivityOptions())
	ctx = wfutils.WithChildSearchAttributes(ctx, params.VXID)

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

	tempFolder, err := wfutils.GetWorkflowTempFolder(ctx)
	if err != nil {
		return nil, err
	}

	subtitlesOutputDir := tempFolder.Append("subtitles")
	err = wfutils.CreateFolder(ctx, subtitlesOutputDir)
	if err != nil {
		return nil, err
	}

	// workflow.Now is stable across replays; time.Now would name the output file
	// differently every time the workflow is replayed.
	titleWithShort := badChars.ReplaceAllString(exportData.Title, "_") + "_short_" + workflow.Now(ctx).Format("20060102150405")

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

	// Everything below assumes a video: the scene-detect call takes VideoFile
	// directly, and both Linux() and the CropShort input dereference it. paths.Path
	// has value receivers, so even the method calls panic on a nil pointer, which
	// fails the workflow task into an endless retry.
	if clipResult.VideoFile == nil {
		return nil, temporal.NewNonRetryableApplicationError(
			"cannot generate a short without a video file", "NO_VIDEO_FILE", nil)
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

	keyframes, err := waitForShortJob(ctx, checkStatusParams)
	if err != nil {
		return nil, err
	}

	shortVideoPath := outputPath.Append(titleWithShort + "_cropped.mov")

	/*
		var subtitlePaths *paths.Path
		if s, ok := clipResult.SubtitleFiles["no"]; ok {
			subtitlePaths = &s
		} else if s, ok := clipResult.SubtitleFiles["und"]; ok {
			subtitlePaths = &s
		}
	*/

	// Get Norwegian audio file if available
	var norwegianAudioPath *paths.Path
	if audioPath, ok := clipResult.AudioFiles["nor"]; ok {
		norwegianAudioPath = &audioPath
	}

	var cropRes activities.CropShortResult
	err = wfutils.Execute(ctx,
		activities.Util.CropShortActivity,
		activities.CropShortInput{
			InputVideoPath:  *clipResult.VideoFile,
			OutputVideoPath: shortVideoPath,
			//SubtitlePath:    subtitlePaths, // For now disable subtitle burn-in
			AudioPath:    norwegianAudioPath,
			KeyFrames:    keyframes,
			InSeconds:    params.InSeconds,
			OutSeconds:   params.OutSeconds,
			SceneChanges: sceneResult,
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

const (
	// shortJobPollInterval is how often the short service is asked whether the
	// job is done, once it has been running longer than shortJobFastPollFor.
	shortJobPollInterval = 30 * time.Second
	// shortJobFastPollFor is the initial window polled every five seconds, so a
	// job that finishes quickly is noticed quickly.
	shortJobFastPollFor  = 2 * time.Minute
	shortJobFastInterval = 5 * time.Second
	// shortJobTimeout bounds the wait. Reaching it fails the workflow with a
	// legible error instead of letting the poll loop grow the history until the
	// server terminates the execution for exceeding its event limit.
	shortJobTimeout = 2 * time.Hour
)

// waitForShortJob polls the short service until the job finishes.
//
// Each pass costs an activity and a timer — five history events — so a fixed
// five second interval writes roughly 3,600 events an hour. Backing off after
// the first couple of minutes and giving up at shortJobTimeout keeps a stuck
// job well inside the history limit, and turns "the server killed the
// execution" into an error naming the job.
func waitForShortJob(ctx workflow.Context, params activities.CheckJobStatusInput) ([]activities.Keyframe, error) {
	logger := workflow.GetLogger(ctx)
	start := workflow.Now(ctx)

	for {
		statusResult, err := wfutils.Execute(ctx, activities.Util.CheckJobStatusActivity, params).Result(ctx)
		if err != nil {
			logger.Error("Failed to check job status: " + err.Error())
			return nil, err
		}

		if statusResult.Status == "completed" {
			logger.Info("Job completed successfully")
			return statusResult.Keyframes, nil
		}

		if statusResult.Status != "in_progress" {
			return nil, fmt.Errorf("job failed with status: %s", statusResult.Status)
		}

		waited := workflow.Now(ctx).Sub(start)
		if waited >= shortJobTimeout {
			return nil, temporal.NewNonRetryableApplicationError(
				fmt.Sprintf("short job %s still in_progress after %s", params.JobID, shortJobTimeout),
				"ShortJobTimeout", nil)
		}

		interval := shortJobPollInterval
		if waited < shortJobFastPollFor {
			interval = shortJobFastInterval
		}

		if err := workflow.Sleep(ctx, interval); err != nil {
			return nil, err
		}
	}
}
