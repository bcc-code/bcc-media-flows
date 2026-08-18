package export

import (
	"fmt"

	"github.com/bcc-code/bcc-media-flows/activities"
	avidispine "github.com/bcc-code/bcc-media-flows/activities/vidispine"
	"github.com/bcc-code/bcc-media-flows/paths"
	"github.com/bcc-code/bcc-media-flows/services/telegram"
	"github.com/bcc-code/bcc-media-flows/utils"
	wfutils "github.com/bcc-code/bcc-media-flows/utils/workflows"
	"github.com/orsinium-labs/enum"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

type IsilonExportParams struct {
	VXID          string
	WatermarkPath string
	Language      string
	AudioSource   string
	Resolution    utils.Resolution
	ExportFormat  string
}

type IsilonExportFormat enum.Member[string]

var (
	IsilonExportFormatProRes422HQ = IsilonExportFormat{Value: "prores_422_hq"}
	IsilonExportFormats           = enum.New(IsilonExportFormatProRes422HQ)
)

func IsilonExport(ctx workflow.Context, params IsilonExportParams) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting IsilonExport")

	ctx = workflow.WithActivityOptions(ctx, wfutils.GetDefaultActivityOptions())

	exportFormat := IsilonExportFormats.Parse(params.ExportFormat)
	if exportFormat == nil {
		return fmt.Errorf("invalid export format: %s", params.ExportFormat)
	}

	selectedLanguages := []string{params.Language}

	data, err := wfutils.Execute(ctx, avidispine.Vidispine.GetExportDataActivity, avidispine.GetExportDataParams{
		VXID:        params.VXID,
		Languages:   selectedLanguages,
		AudioSource: params.AudioSource,
	}).Result(ctx)

	if err != nil {
		return err
	}

	wfutils.SendTelegramText(ctx,
		telegram.ChatOther,
		fmt.Sprintf(
			"🟦 Export of `%s` started.\nTitle: `%s`\nDestinations: isilon\n\nRunID: `%s`",
			params.VXID,
			data.Title,
			workflow.GetInfo(ctx).OriginalRunID,
		),
	)

	logger.Info("Retrieved data from vidispine")

	tempDir, err := wfutils.GetWorkflowTempFolder(ctx)
	if err != nil {
		return err
	}

	outputDir := wfutils.GetIsilonExportFolder(ctx, data.SafeTitle)

	subtitlesOutputDir := outputDir.Append("subtitles")
	err = wfutils.CreateFolder(ctx, subtitlesOutputDir)
	if err != nil {
		return err
	}

	ctx = workflow.WithChildOptions(ctx, wfutils.GetVXDefaultWorkflowOptions(ctx, params.VXID))

	mergeResult, err := wfutils.FutureResult[*MergeExportDataResult](ctx, workflow.ExecuteChildWorkflow(ctx, MergeExportData, MergeExportDataParams{
		ExportData:       data,
		TempDir:          tempDir,
		SubtitlesDir:     subtitlesOutputDir,
		MakeVideo:        true,
		MakeAudio:        true,
		MakeSubtitles:    false,
		MakeTranscript:   false,
		Languages:        selectedLanguages,
		OriginalLanguage: data.OriginalLanguage,
	}))

	if err != nil {
		wfutils.SendTelegramError(ctx, telegram.ChatOther, params.VXID, err)
		return err
	}

	audioPaths := []paths.Path{}
	audioKeys, err := wfutils.GetMapKeysSafely(ctx, mergeResult.AudioFiles)
	if err != nil {
		wfutils.SendTelegramError(ctx, telegram.ChatOther, params.VXID, err)
		return err
	}

	for _, key := range audioKeys {
		audioPaths = append(audioPaths, mergeResult.AudioFiles[key])
	}

	// Audio-only items have no VideoFile (VXExport sets MakeVideo from
	// fileInfo.HasVideo), and dereferencing it would panic the workflow task into an
	// endless retry loop.
	if mergeResult.VideoFile == nil {
		err = temporal.NewNonRetryableApplicationError(
			"Isilon export needs a video file, but this item is audio-only", "NO_VIDEO_FILE", nil)
		wfutils.SendTelegramError(ctx, telegram.ChatOther, params.VXID, err)
		return err
	}

	switch exportFormat.Value {
	case IsilonExportFormatProRes422HQ.Value:
		videoResult, err := wfutils.Execute(ctx, activities.Video.TranscodeToProResActivity, activities.EncodeParams{
			FilePath:       *mergeResult.VideoFile,
			AudioPaths:     audioPaths,
			OutputDir:      outputDir,
			Resolution:     &params.Resolution,
			FrameRate:      50,
			Interlace:      false,
			BurnInSubtitle: nil,
			SubtitleStyle:  nil,
			Alpha:          false,
		}).Result(ctx)

		if err != nil {
			return err
		}

		wfutils.SendTelegramText(ctx, telegram.ChatOther, fmt.Sprintf("🟩 Export of `%s` completed:\n```\n%s\n```", params.VXID, videoResult.OutputPath.Linux()))

	default:
		return fmt.Errorf("invalid export format: %s", exportFormat.Value)
	}

	return nil
}
