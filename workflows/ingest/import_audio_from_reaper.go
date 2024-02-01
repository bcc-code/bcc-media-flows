package ingestworkflows

import (
	"fmt"
	"strconv"
	"strings"

	bccmflows "github.com/bcc-code/bcc-media-flows"
	"github.com/bcc-code/bcc-media-flows/activities"
	"github.com/bcc-code/bcc-media-flows/activities/cantemo"
	vsactivity "github.com/bcc-code/bcc-media-flows/activities/vidispine"
	"github.com/bcc-code/bcc-media-flows/common"
	"github.com/bcc-code/bcc-media-flows/paths"
	wfutils "github.com/bcc-code/bcc-media-flows/utils/workflows"
	"github.com/bcc-code/bcc-media-flows/workflows"
	"go.temporal.io/sdk/workflow"
)

type ImportAudioFileFromReaperParams struct {
	Path      string
	VideoVXID string
	BaseName  string
}

func ImportAudioFileFromReaper(ctx workflow.Context, params ImportAudioFileFromReaperParams) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting import of audio file from Reaper")

	ctx = workflow.WithActivityOptions(ctx, wfutils.GetDefaultActivityOptions())

	inputFile := paths.MustParse(params.Path)

	fileOK := false
	err := wfutils.ExecuteWithQueue(ctx, activities.WaitForFile, activities.FileInput{
		Path: inputFile,
	}).Get(ctx, &fileOK)
	if err != nil {
		return err
	}

	if !fileOK {
		return fmt.Errorf("File %s is reported not OK by the system", inputFile)
	}

	tempFolder, _ := wfutils.GetWorkflowTempFolder(ctx)
	tempFile := tempFolder.Append(inputFile.Base())
	err = wfutils.CopyFile(ctx, inputFile, tempFile)
	if err != nil {
		return err
	}

	isSilent := false
	err = wfutils.ExecuteWithQueue(ctx, activities.DetectSilence, common.AudioInput{
		Path:            tempFile,
		DestinationPath: tempFolder,
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
		wfutils.NotifyTelegramChannel(ctx, fmt.Sprintf("File %s is silent, skipping", bccmflows.LanguagesByReaper[reaperTrackNumber].LanguageName))
		return fmt.Errorf("File %s is silent, skipping", tempFile.Base())
	}

	outputFolder, err := wfutils.GetWorkflowRawOutputFolder(ctx)
	if err != nil {
		return err
	}

	getFileResult := vsactivity.GetFileFromVXResult{}
	err = wfutils.ExecuteWithQueue(ctx, vsactivity.GetFileFromVXActivity, vsactivity.GetFileFromVXParams{
		VXID: params.VideoVXID,
		Tags: []string{"original"},
	}).Get(ctx, &getFileResult)
	if err != nil {
		return err
	}

	// Generate a filename withe the language code
	outPath := outputFolder.Append(fmt.Sprintf("%s-%s.wav", params.BaseName, strings.ToUpper(bccmflows.LanguagesByReaper[reaperTrackNumber].ISO6391)))
	err = wfutils.ExecuteWithQueue(ctx, activities.AdjustAudioToVideoStart, activities.AdjustAudioToVideoStartInput{
		AudioFile:  tempFile,
		VideoFile:  getFileResult.FilePath,
		OutputFile: outPath,
	}).Get(ctx, nil)
	if err != nil {
		return err
	}

	// Create placeholder
	var assetResult vsactivity.CreatePlaceholderResult
	err = wfutils.ExecuteWithQueue(ctx, vsactivity.CreatePlaceholderActivity, vsactivity.CreatePlaceholderParams{
		Title: outPath.Base(),
	}).Get(ctx, &assetResult)
	if err != nil {
		return err
	}

	// Ingest to placeholder
	err = wfutils.ExecuteWithQueue(ctx, vsactivity.AddFileToPlaceholder, vsactivity.AddFileToPlaceholderParams{
		FilePath: outPath,
		AssetID:  assetResult.AssetID,
		Growing:  false,
	}).Get(ctx, nil)
	if err != nil {
		return err
	}

	// Add relation
	err = wfutils.ExecuteWithQueue(ctx, cantemo.AddRelation, cantemo.AddRelationParams{
		Child:  assetResult.AssetID,
		Parent: params.VideoVXID,
	}).Get(ctx, nil)

	if err != nil {
		return err
	}

	err = workflow.ExecuteChildWorkflow(ctx, workflows.TranscodePreviewVX, workflows.TranscodePreviewVXInput{
		VXID: assetResult.AssetID,
	}).Get(ctx, nil)

	return err
}