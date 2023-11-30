package ingestworkflows

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/bcc-code/bccm-flows/activities"
	batonactivities "github.com/bcc-code/bccm-flows/activities/baton"
	"github.com/bcc-code/bccm-flows/common"
	"github.com/bcc-code/bccm-flows/paths"
	"github.com/bcc-code/bccm-flows/services/baton"
	"github.com/bcc-code/bccm-flows/services/ingest"
	"github.com/bcc-code/bccm-flows/services/notifications"
	"github.com/bcc-code/bccm-flows/services/vidispine/vscommon"
	"github.com/bcc-code/bccm-flows/utils"
	"github.com/bcc-code/bccm-flows/utils/workflows"
	"go.temporal.io/sdk/workflow"
)

type MasterParams struct {
	Targets  []notifications.Target
	Metadata *ingest.Metadata

	OrderForm  OrderForm
	Directory  paths.Path
	OutputDir paths.Path
	SourceFile *paths.Path
}

type MasterResult struct {
	Report        *baton.QCReport
	AssetID       string
	AnalyzeResult *common.AnalyzeEBUR128Result
	Path          paths.Path
}

// regexp for making sure the filename does not contain non-alphanumeric characters
var nonAlphanumeric = regexp.MustCompile("[^a-zA-Z0-9_]")

func Masters(ctx workflow.Context, params MasterParams) (*MasterResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting VBMaster workflow")

	ctx = workflow.WithActivityOptions(ctx, wfutils.GetDefaultActivityOptions())

	result, err := uploadMaster(ctx, params)
	if err != nil {
		return nil, err
	}

	if utils.IsMedia(result.Path.Local()) {
		// This isn't run on VB masters in old system, but see no reason to not run it here.
		result.AnalyzeResult, err = analyzeAudioAndSetMetadata(ctx, result.AssetID, result.Path)
		if err != nil {
			return nil, err
		}

		if result.Report.TopLevelInfo.Error == 0 {
			err = transcodeAndTranscribe(ctx, []string{result.AssetID}, params.Metadata.JobProperty.Language)
			if err != nil {
				return nil, err
			}
		}
	}

	return result, nil
}

func uploadMaster(ctx workflow.Context, params MasterParams) (*MasterResult, error) {
	var filename string
	var err error
	switch params.OrderForm {
	case OrderFormOtherMaster, OrderFormVBMaster, OrderFormSeriesMaster, OrderFormLEDMaterial, OrderFormPodcast:
		filename, err = masterFilename(params.Metadata.JobProperty)
	default:
		return nil, fmt.Errorf("unsupported order form: %s", params.OrderForm)
	}
	if err != nil {
		return nil, err
	}

	sourceFile := params.SourceFile
	if sourceFile == nil {
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
		sourceFile = &files[0]
	}

	file := params.OutputDir.Append(filename)
	err = wfutils.MoveFile(ctx, *sourceFile, file)
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

	var report *baton.QCReport
	if utils.IsMedia(file.Local()) {
		plan := baton.TestPlanMXF
		if filepath.Ext(file.Base()) == ".mov" {
			plan = baton.TestPlanMOV
		}
		err = wfutils.ExecuteWithQueue(ctx, batonactivities.QC, batonactivities.QCParams{
			Path: file,
			Plan: plan,
		}).Get(ctx, &report)
		if err != nil {
			return nil, err
		}
	}

	err = notifyImportCompleted(ctx, params.Targets, params.Metadata.JobProperty.JobID, map[string]paths.Path{
		result.AssetID: file,
	})
	if err != nil {
		return nil, err
	}

	return &MasterResult{
		Report:  report,
		AssetID: result.AssetID,
		Path:    file,
	}, nil
}

func analyzeAudioAndSetMetadata(ctx workflow.Context, assetID string, path paths.Path) (*common.AnalyzeEBUR128Result, error) {
	var result common.AnalyzeEBUR128Result
	err := wfutils.ExecuteWithQueue(ctx, activities.AnalyzeEBUR128Activity, activities.AnalyzeEBUR128Params{
		FilePath:       path,
		TargetLoudness: -24,
	}).Get(ctx, &result)
	if err != nil {
		return nil, err
	}

	values := map[string]float64{
		vscommon.FieldLoudnessLUFS.Value:  result.IntegratedLoudness,
		vscommon.FieldTruePeak.Value:      result.TruePeak,
		vscommon.FieldLoudnessRange.Value: result.LoudnessRange,
	}

	keys, err := wfutils.GetMapKeysSafely(ctx, values)
	if err != nil {
		return nil, err
	}
	for _, key := range keys {
		err = wfutils.SetVidispineMeta(ctx, assetID, key, strconv.FormatFloat(values[key], 'f', 2, 64))
		if err != nil {
			return nil, err
		}
	}

	return &result, nil
}

func masterFilename(props ingest.JobProperty) (string, error) {
	var parts []string
	if props.ProgramID != "" {
		parts = append(parts, strings.Split(props.ProgramID, " ")[0])
	}
	if props.ProgramPost != "" {
		parts = append(parts, props.ProgramPost)
	}
	parts = append(parts, strings.ToUpper(props.ReceivedFilename))
	if props.AssetType != "" {
		parts = append(parts, props.AssetType)
	}
	if props.Language != "" {
		parts = append(parts, strings.ToUpper(props.Language))
	}

	filename := strings.Join(parts, "_")
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
