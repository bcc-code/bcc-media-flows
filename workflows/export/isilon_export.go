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

	date := workflow.Now(ctx)
	id := workflow.GetInfo(ctx).OriginalRunID
	outputDir := paths.Path{
		Drive: paths.IsilonDrive,
		Path:  fmt.Sprintf("Export/%s/%s", date.Format("2006-01"), data.SafeTitle+"-"+id),
	}

	subtitlesOutputDir := outputDir.Append("subtitles")
	err = wfutils.CreateFolder(ctx, subtitlesOutputDir)
	if err != nil {
		return err
	}

	ctx = workflow.WithChildOptions(ctx, wfutils.GetVXDefaultWorkflowOptions(params.VXID))

	var mergeResult MergeExportDataResult
	err = workflow.ExecuteChildWorkflow(ctx, MergeExportData, MergeExportDataParams{
		ExportData:       data,
		TempDir:          tempDir,
		SubtitlesDir:     subtitlesOutputDir,
		MakeVideo:        true,
		MakeAudio:        true,
		MakeSubtitles:    false,
		MakeTranscript:   false,
		Languages:        selectedLanguages,
		OriginalLanguage: data.OriginalLanguage,
	}).Get(ctx, &mergeResult)

	if err != nil {
		wfutils.SendTelegramText(ctx, telegram.ChatVOD, fmt.Sprintf("🟥 Export of `%s` failed:\n```\n%s\n```", params.VXID, err.Error()))
		return err
	}

	audioPaths := []paths.Path{}
	for _, audioPath := range mergeResult.AudioFiles {
		audioPaths = append(audioPaths, audioPath)
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

		wfutils.SendTelegramText(ctx, telegram.ChatVOD, fmt.Sprintf("🟩 Export of `%s` completed:\n```\n%s\n```", params.VXID, videoResult.OutputPath.Linux()))

	default:
		return fmt.Errorf("invalid export format: %s", exportFormat.Value)
	}

	return nil
}
