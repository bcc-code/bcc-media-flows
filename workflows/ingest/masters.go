package ingestworkflows

import (
	"fmt"
	"github.com/bcc-code/bccm-flows/activities"
	batonactivities "github.com/bcc-code/bccm-flows/activities/baton"
	"github.com/bcc-code/bccm-flows/common"
	"github.com/bcc-code/bccm-flows/services/baton"
	"github.com/bcc-code/bccm-flows/services/ingest"
	"github.com/bcc-code/bccm-flows/services/vidispine/vscommon"
	"github.com/bcc-code/bccm-flows/utils"
	"github.com/bcc-code/bccm-flows/utils/wfutils"
	"go.temporal.io/sdk/workflow"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

type MasterParams struct {
	Metadata *ingest.Metadata

	OrderForm OrderForm
	Directory string
}

type MasterResult struct {
	Report        baton.QCReport
	AssetID       string
	AnalyzeResult *common.AnalyzeEBUR128Result
}

// regexp for making sure the filename does not contain non-alphanumeric characters
var nonAlphanumeric = regexp.MustCompile("[^a-zA-Z0-9_]")

func uploadMaster(ctx workflow.Context, params MasterParams) (*MasterResult, error) {
	var filename string
	var err error
	switch params.OrderForm {
	case OrderFormSeriesMaster:
		filename, err = seriesMasterFilename(params.Metadata)
	case OrderFormVBMaster:
		filename, err = vbMasterFilename(params.Metadata)
	case OrderFormOtherMaster:
		filename, err = otherMasterFilename(params.Metadata)
	}
	if err != nil {
		return nil, err
	}

	files, err := wfutils.ListFiles(ctx, params.Directory)
	if err != nil {
		return nil, err
	}

	if len(files) == 0 {
		return nil, fmt.Errorf("no files in directory: %s", params.Directory)
	}
	if len(files) > 1 {
		return nil, fmt.Errorf("too many files in directory: %s", params.Directory)
	}

	outputDir, err := wfutils.GetWorkflowMastersOutputFolder(ctx)
	if err != nil {
		return nil, err
	}

	file := filepath.Join(outputDir, filename)
	err = wfutils.MoveFile(ctx, files[0], file)
	if err != nil {
		return nil, err
	}

	result, err := importFileAsTag(ctx, "original", file, filename)
	if err != nil {
		return nil, err
	}

	err = addMetaTags(ctx, result.AssetID, params.Metadata)
	if err != nil {
		return nil, err
	}

	err = wfutils.WaitForVidispineJob(ctx, result.ImportJobID)
	if err != nil {
		return nil, err
	}

	path, err := utils.ParsePath(file)
	if err != nil {
		return nil, err
	}

	plan := baton.TestPlanMXF
	if strings.HasSuffix(file, ".mov") {
		plan = baton.TestPlanMOV
	}

	var report baton.QCReport
	err = wfutils.ExecuteWithQueue(ctx, batonactivities.QC, batonactivities.QCParams{
		Path: path,
		Plan: plan,
	}).Get(ctx, &report)
	if err != nil {
		return nil, err
	}

	return &MasterResult{
		Report:  report,
		AssetID: result.AssetID,
	}, nil
}

func Masters(ctx workflow.Context, params MasterParams) (*MasterResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting VBMaster workflow")

	ctx = workflow.WithActivityOptions(ctx, wfutils.GetDefaultActivityOptions())

	result, err := uploadMaster(ctx, params)
	if err != nil {
		return nil, err
	}

	var analyzeResult common.AnalyzeEBUR128Result
	err = wfutils.ExecuteWithQueue(ctx, activities.AnalyzeEBUR128Activity, activities.AnalyzeEBUR128Params{}).Get(ctx, &analyzeResult)
	if err != nil {
		return nil, err
	}

	result.AnalyzeResult = &analyzeResult

	if result.Report.TopLevelInfo.Error == 0 {
		err = postImportActions(ctx, []string{result.AssetID}, params.Metadata.JobProperty.Language)
		if err != nil {
			return nil, err
		}
	}

	return result, nil
}

func VBMaster(ctx workflow.Context, params MasterParams) (*MasterResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting VBMaster workflow")

	ctx = workflow.WithActivityOptions(ctx, wfutils.GetDefaultActivityOptions())

	result, err := uploadMaster(ctx, params)
	if err != nil {
		return nil, err
	}

	if result.Report.TopLevelInfo.Error == 0 {
		err = postImportActions(ctx, []string{result.AssetID}, params.Metadata.JobProperty.Language)
		if err != nil {
			return nil, err
		}
	}

	return result, nil
}

func seriesMasterFilename(metadata *ingest.Metadata) (string, error) {
	programID := metadata.JobProperty.ProgramID
	if programID != "" {
		programID = strings.Split(programID, " ")[0]
	}

	filename := programID
	filename += "_" + strings.ToUpper(metadata.JobProperty.ReceivedFilename)
	filename += "_" + metadata.JobProperty.AssetType
	filename += "_" + strings.ToUpper(metadata.JobProperty.Language)

	filename = strings.ReplaceAll(filename, " ", "_")

	if nonAlphanumeric.MatchString(filename) {
		return "", fmt.Errorf("filename contains non-alphanumeric characters: %s", filename)
	}

	return filename, nil
}

func otherMasterFilename(metadata *ingest.Metadata) (string, error) {
	programID := metadata.JobProperty.ProgramID
	if programID != "" {
		programID = strings.Split(programID, " ")[0]
	}

	filename := programID

	filename += "_" + strings.ToUpper(metadata.JobProperty.ReceivedFilename)
	filename += "_" + metadata.JobProperty.AssetType
	filename += "_" + strings.ToUpper(metadata.JobProperty.Language)

	filename = strings.ReplaceAll(filename, " ", "_")

	if nonAlphanumeric.MatchString(filename) {
		return "", fmt.Errorf("filename contains non-alphanumeric characters: %s", filename)
	}

	return filename, nil
}

func vbMasterFilename(metadata *ingest.Metadata) (string, error) {
	programID := metadata.JobProperty.ProgramID
	if programID != "" {
		programID = strings.Split(programID, " ")[0]
	}

	filename := programID
	if metadata.JobProperty.ProgramPost != "" {
		filename += "_" + strings.ToUpper(metadata.JobProperty.ProgramPost)
	}
	filename += "_" + strings.ToUpper(metadata.JobProperty.ReceivedFilename)

	filename = strings.ReplaceAll(filename, " ", "_")

	if nonAlphanumeric.MatchString(filename) {
		return "", fmt.Errorf("filename contains non-alphanumeric characters: %s", filename)
	}

	return filename, nil
}

func addMetaTags(ctx workflow.Context, assetID string, metadata *ingest.Metadata) error {
	err := wfutils.SetVidispineMeta(ctx, assetID, vscommon.FieldUploadedBy.Value, metadata.JobProperty.SenderEmail)
	if err != nil {
		return err
	}

	err = wfutils.SetVidispineMeta(ctx, assetID, vscommon.FieldUploadJob.Value, strconv.Itoa(metadata.JobProperty.JobID))
	if err != nil {
		return err
	}

	if metadata.JobProperty.PersonsAppearing != "" {
		for _, person := range strings.Split(metadata.JobProperty.PersonsAppearing, ",") {
			person = strings.TrimSpace(person)
			if person == "" {
				continue
			}
			err = wfutils.AddVidispineMetaValue(ctx, assetID, vscommon.FieldPersonsAppearing.Value, person)
			if err != nil {
				return err
			}
		}
	}

	if metadata.JobProperty.Tags != "" {
		for _, tag := range strings.Split(metadata.JobProperty.Tags, ",") {
			tag = strings.TrimSpace(tag)
			if tag == "" {
				continue
			}
			err = wfutils.AddVidispineMetaValue(ctx, assetID, vscommon.FieldGeneralTags.Value, tag)
			if err != nil {
				return err
			}
		}
	}

	if metadata.JobProperty.Language != "" {
		err = wfutils.SetVidispineMeta(ctx, assetID, vscommon.FieldLanguagesRecorded.Value, metadata.JobProperty.Language)
		if err != nil {
			return err
		}
	}

	// let workflow panic if the format is invalid?
	program := strings.Split(metadata.JobProperty.ProgramID, " - ")[1]
	if program != "" {
		err = wfutils.SetVidispineMeta(ctx, assetID, vscommon.FieldProgram.Value, program)
		if err != nil {
			return err
		}
	}

	if metadata.JobProperty.Season != "" {
		err = wfutils.SetVidispineMeta(ctx, assetID, vscommon.FieldSeason.Value, metadata.JobProperty.Season)
		if err != nil {
			return err
		}
	}

	if metadata.JobProperty.Episode != "" {
		err = wfutils.SetVidispineMeta(ctx, assetID, vscommon.FieldEpisode.Value, metadata.JobProperty.Episode)
		if err != nil {
			return err
		}
	}

	if metadata.JobProperty.EpisodeTitle != "" {
		title := program + " | " + metadata.JobProperty.EpisodeTitle

		err = wfutils.SetVidispineMeta(ctx, assetID, vscommon.FieldTitle.Value, title)
		if err != nil {
			return err
		}
	}

	if metadata.JobProperty.EpisodeDescription != "" {
		err = wfutils.SetVidispineMeta(ctx, assetID, vscommon.FieldEpisodeDescription.Value, metadata.JobProperty.EpisodeDescription)
		if err != nil {
			return err
		}
	}

	return nil
}
