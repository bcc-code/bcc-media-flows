package ingestworkflows

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/bcc-code/bcc-media-flows/services/telegram"
	miscworkflows "github.com/bcc-code/bcc-media-flows/workflows/misc"

	bccmflows "github.com/bcc-code/bcc-media-flows"
	"github.com/bcc-code/bcc-media-flows/activities"
	"github.com/bcc-code/bcc-media-flows/activities/cantemo"
	vsactivity "github.com/bcc-code/bcc-media-flows/activities/vidispine"
	"github.com/bcc-code/bcc-media-flows/common"
	"github.com/bcc-code/bcc-media-flows/paths"
	"github.com/bcc-code/bcc-media-flows/services/vidispine/vscommon"
	wfutils "github.com/bcc-code/bcc-media-flows/utils/workflows"
	"go.temporal.io/api/enums/v1"
	"go.temporal.io/sdk/workflow"
)

type RelateAudioToVideoParams struct {
	VideoVXID    string
	AudioList    map[string]paths.Path
	PreviewDelay time.Duration
}

func RelateAudioToVideo(ctx workflow.Context, params RelateAudioToVideoParams) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting RelateAudioToVideo activity")

	ctx = workflow.WithActivityOptions(ctx, wfutils.GetDefaultActivityOptions())
	previewOpts := workflow.GetChildWorkflowOptions(ctx)
	previewOpts.ParentClosePolicy = enums.PARENT_CLOSE_POLICY_ABANDON
	previewCtx := workflow.WithChildOptions(ctx, previewOpts)

	langs, err := wfutils.GetMapKeysSafely(ctx, params.AudioList)
	if err != nil {
		return err
	}

	for _, lang := range langs {
		path := params.AudioList[lang]
		// Create placeholder
		var assetResult vsactivity.CreatePlaceholderResult
		err := wfutils.Execute(ctx, activities.Vidispine.CreatePlaceholderActivity, vsactivity.CreatePlaceholderParams{
			Title: path.Base(),
		}).Get(ctx, &assetResult)
		if err != nil {
			return err
		}

		// Ingest to placeholder
		err = wfutils.Execute(ctx, activities.Vidispine.AddFileToPlaceholder, vsactivity.AddFileToPlaceholderParams{
			FilePath: path,
			AssetID:  assetResult.AssetID,
			Growing:  false,
		}).Get(ctx, nil)
		if err != nil {
			return err
		}

		// We dow *not* wait for preview to be ready
		workflow.ExecuteChildWorkflow(previewCtx, miscworkflows.TranscodePreviewVX, miscworkflows.TranscodePreviewVXInput{
			VXID:  assetResult.AssetID,
			Delay: params.PreviewDelay,
		})

		// Add relation
		err = wfutils.Execute(ctx, cantemo.AddRelation, cantemo.AddRelationParams{
			Child:  assetResult.AssetID,
			Parent: params.VideoVXID,
		}).Get(ctx, nil)
		if err != nil {
			return err
		}

		err = wfutils.Execute(ctx, activities.Vidispine.SetVXMetadataFieldActivity, vsactivity.VXMetadataFieldParams{
			ItemID:  params.VideoVXID,
			GroupID: "System",
			Key:     bccmflows.LanguagesByISO[lang].RelatedMBFieldID,
			Value:   assetResult.AssetID,
		}).Get(ctx, nil)
		if err != nil {
			logger.Error(fmt.Sprintf("SetVXMetadataFieldActivity: %s", err.Error()))
		}

		err = wfutils.Execute(ctx, activities.Vidispine.SetVXMetadataFieldActivity, vsactivity.VXMetadataFieldParams{
			ItemID: assetResult.AssetID,
			Key:    vscommon.FieldLanguagesRecorded.Value,
			Value:  lang,
		}).Get(ctx, nil)

		if err != nil {
			return err
		}
	}

	return nil
}

type ImportAudioFileFromReaperParams struct {
	Path       string
	VideoVXID  string
	BaseName   string
	OutputPath paths.Path
}

func ImportAudioFileFromReaper(ctx workflow.Context, params ImportAudioFileFromReaperParams) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting import of audio file from Reaper")

	ctx = workflow.WithActivityOptions(ctx, wfutils.GetDefaultActivityOptions())

	err := doImportAudioFileFromReaper(ctx, params)

	if err != nil {
		wfutils.SendTelegramText(ctx, telegram.ChatOther, fmt.Sprintf("🟥 Import of audio file from Reaper failed: ```%s```", err.Error()))
		return err
	}
	return nil
}

func doImportAudioFileFromReaper(ctx workflow.Context, params ImportAudioFileFromReaperParams) error {
	inputFile := paths.MustParse(params.Path)

	fileOK, err := wfutils.Execute(ctx, activities.Util.WaitForFile, activities.FileInput{
		Path: inputFile,
	}).Result(ctx)
	if err != nil {
		return err
	}

	if !fileOK {
		return fmt.Errorf("file %s is reported not OK by the system", inputFile)
	}

	tempFolder, _ := wfutils.GetWorkflowTempFolder(ctx)
	tempFile := tempFolder.Append(inputFile.Base())
	err = wfutils.CopyFile(ctx, inputFile, tempFile)
	if err != nil {
		return err
	}

	isSilent := false
	err = wfutils.Execute(ctx, activities.Audio.DetectSilence, common.DetectSilenceInput{
		Path: tempFile,
	}).Get(ctx, &isSilent)

	if err != nil {
		return err
	}

	// ReaperTrack-DATE_TIME.wav
	// 22-240122_1526.wav
	reaperTrackNumber, err := strconv.Atoi(strings.Split(tempFile.Base(), "-")[0])
	if err != nil {
		return err
	}

	if isSilent {
		wfutils.SendTelegramText(ctx, telegram.ChatOther, fmt.Sprintf("🟧 File %s is silent, skipping", bccmflows.LanguagesByReaper[reaperTrackNumber].LanguageName))

		// This is not a fail, so we should not send an error
		return nil
	}

	outputFolder := params.OutputPath

	getFileResult := vsactivity.GetFileFromVXResult{}
	err = wfutils.Execute(ctx, activities.Vidispine.GetFileFromVXActivity, vsactivity.GetFileFromVXParams{
		VXID: params.VideoVXID,
		Tags: []string{"original"},
	}).Get(ctx, &getFileResult)
	if err != nil {
		return err
	}

	lang := bccmflows.LanguagesByReaper[reaperTrackNumber]

	// Generate a filename with the language code
	outPath := outputFolder.Append(fmt.Sprintf("%s-%s.wav", params.BaseName, strings.ToUpper(lang.ISO6391)))
	err = wfutils.Execute(ctx, activities.Audio.AdjustAudioToVideoStart, activities.AdjustAudioToVideoStartInput{
		AudioFile:  tempFile,
		VideoFile:  getFileResult.FilePath,
		OutputFile: outPath,
	}).Wait(ctx)
	if err != nil {
		return err
	}

	return RelateAudioToVideo(ctx, RelateAudioToVideoParams{
		AudioList: map[string]paths.Path{
			lang.ISO6391: outPath,
		},
		PreviewDelay: 2 * time.Hour,
		VideoVXID:    params.VideoVXID,
	})
}
