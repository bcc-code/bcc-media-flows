package ingestworkflows

import (
	"fmt"
	"strconv"
	"strings"

	bccmflows "github.com/bcc-code/bcc-media-flows"
	"github.com/bcc-code/bcc-media-flows/activities"
	vsactivity "github.com/bcc-code/bcc-media-flows/activities/vidispine"
	"github.com/bcc-code/bcc-media-flows/common"
	"github.com/bcc-code/bcc-media-flows/paths"
	wfutils "github.com/bcc-code/bcc-media-flows/utils/workflows"
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
	isSilent := false
	err = wfutils.ExecuteWithQueue(ctx, activities.DetectSilence, common.AudioInput{
		Path:            inputFile,
		DestinationPath: tempFolder,
	}).Get(ctx, &isSilent)

	if err != nil {
		return err
	}

	if isSilent {
		wfutils.NotifyTelegramChannel(ctx, fmt.Sprintf("File %s is silent, skipping", inputFile.Base()))
		return fmt.Errorf("File %s is silent, skipping", inputFile.Base())
	}

	// ReaperTrack-DATE_TIME.wav
	// 22-240122_1526.wav
	reaperTrackNumber, err := strconv.Atoi(strings.Split(inputFile.Base(), ".")[0])
	if err != nil {
		return err
	}

	outputFolder, err := wfutils.GetWorkflowRawOutputFolder(ctx)
	if err != nil {
		return err
	}

	// Generate a filename withe the language code
	outPath := outputFolder.Append(fmt.Sprintf("%s-%s.wav", params.BaseName, strings.ToUpper(bccmflows.LanguagesByReaper[reaperTrackNumber].ISO6391)))
	err = wfutils.ExecuteWithQueue(ctx, activities.PrependSilence, activities.PrependSilenceInput{
		FilePath: inputFile,
		Output:   outPath,
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
	err = wfutils.ExecuteWithQueue(ctx, vsactivity.AddRelation, vsactivity.AddRelationParams{
		Child:  assetResult.AssetID,
		Parent: params.VideoVXID,
	}).Get(ctx, nil)

	return err
}
