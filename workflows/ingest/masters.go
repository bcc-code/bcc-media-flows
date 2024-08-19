package ingestworkflows

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/bcc-code/bcc-media-flows/services/rclone"
	miscworkflows "github.com/bcc-code/bcc-media-flows/workflows/misc"
	"github.com/samber/lo"

	"github.com/bcc-code/bcc-media-flows/activities"
	batonactivities "github.com/bcc-code/bcc-media-flows/activities/baton"
	vsactivity "github.com/bcc-code/bcc-media-flows/activities/vidispine"
	"github.com/bcc-code/bcc-media-flows/common"
	"github.com/bcc-code/bcc-media-flows/paths"
	"github.com/bcc-code/bcc-media-flows/services/baton"
	"github.com/bcc-code/bcc-media-flows/services/ingest"
	"github.com/bcc-code/bcc-media-flows/services/vidispine/vscommon"
	"github.com/bcc-code/bcc-media-flows/utils"
	wfutils "github.com/bcc-code/bcc-media-flows/utils/workflows"
	"go.temporal.io/api/enums/v1"
	"go.temporal.io/sdk/workflow"
)

type MasterParams struct {
	Targets  []string
	Metadata *ingest.Metadata

	OrderForm  OrderForm
	Directory  paths.Path
	OutputDir  paths.Path
	SourceFile *paths.Path
}

type MasterResult struct {
	Report        *baton.QCReport
	AnalyzeResult *common.AnalyzeEBUR128Result
	ImportedVXs   map[string]paths.Path
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

	return result, nil
}

func processMaster(ctx workflow.Context, sourceFile paths.Path, destinationFile paths.Path, metadata *ingest.Metadata) (string, error) {
	err := wfutils.MoveFile(ctx, sourceFile, destinationFile, rclone.PriorityNormal)
	if err != nil {
		return "", err
	}

	result, err := ImportFileAsTag(ctx, "original", destinationFile, destinationFile.Base())
	if err != nil {
		return "", err
	}

	err = addMetaTags(ctx, result.AssetID, metadata)
	if err != nil {
		return "", err
	}

	err = wfutils.WaitForVidispineJob(ctx, result.ImportJobID)
	if err != nil {
		return "", err
	}

	var report *baton.QCReport
	if utils.IsMedia(destinationFile.Local()) {
		plan := baton.TestPlanMXF
		if destinationFile.Ext() == ".mov" {
			plan = baton.TestPlanMOV
		}
		err = wfutils.Execute(ctx, batonactivities.QC, batonactivities.QCParams{
			Path: destinationFile,
			Plan: plan,
		}).Get(ctx, &report)
		if err != nil {
			return "", err
		}
	}

	parentAbandonOptions := workflow.GetChildWorkflowOptions(ctx)
	parentAbandonOptions.ParentClosePolicy = enums.PARENT_CLOSE_POLICY_ABANDON
	asyncCtx := workflow.WithChildOptions(ctx, parentAbandonOptions)

	// Trigger transcribe and create previews but don't wait for them to finish
	workflow.ExecuteChildWorkflow(asyncCtx, miscworkflows.TranscribeVX, miscworkflows.TranscribeVXInput{
		VXID:     result.AssetID,
		Language: "no",
	})

	// This just triggers the task, the actual work is done in the background by Vidispine
	_ = wfutils.Execute(ctx, activities.Vidispine.CreateThumbnailsActivity, vsactivity.CreateThumbnailsParams{
		AssetID: result.AssetID,
	}).Get(ctx, nil)

	createPreviewsAsync(ctx, []string{result.AssetID})

	return result.AssetID, nil
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

	sourceFiles := []paths.Path{}

	if params.SourceFile != nil {
		sourceFiles = append(sourceFiles, *params.SourceFile)
	} else {
		files, err := wfutils.ListFiles(ctx, params.Directory)
		if err != nil {
			return nil, err
		}

		if len(files) == 0 {
			return nil, fmt.Errorf("no files in directory: %s", params.Directory)
		}

		if len(files) > 1 && params.OrderForm != OrderFormVBMasterBulk {
			return nil, fmt.Errorf("too many files in directory: %s", params.Directory)
		}

		sourceFiles = files
	}

	importedVXs := map[string]paths.Path{}
	errors := []error{}

	for _, sourceFile := range sourceFiles {
		file := params.OutputDir.Append(sourceFile.Base())

		if filename == "" {
			file = params.OutputDir.Append(filename)
		}

		result, err := processMaster(ctx, sourceFile, file, params.Metadata)

		if err != nil {
			errors = append(errors, err)
			continue
		}

		importedVXs[result] = file
	}

	if len(errors) > 0 {
		errText := lo.Reduce(errors, func(acc string, err error, _ int) string {
			return acc + err.Error() + "\n"
		}, "")

		return nil, fmt.Errorf(errText)
	}

	parentAbandonOptions := workflow.GetChildWorkflowOptions(ctx)
	parentAbandonOptions.ParentClosePolicy = enums.PARENT_CLOSE_POLICY_ABANDON
	asyncCtx := workflow.WithChildOptions(ctx, parentAbandonOptions)
	err = notifyImportCompleted(asyncCtx, params.Targets, params.Metadata.JobProperty.JobID, importedVXs)
	if err != nil {
		return nil, err
	}

	return &MasterResult{
		ImportedVXs: importedVXs,
	}, nil
}

func analyzeAudioAndSetMetadata(ctx workflow.Context, assetID string, path paths.Path) (*common.AnalyzeEBUR128Result, error) {
	var result common.AnalyzeEBUR128Result
	err := wfutils.Execute(ctx, activities.Audio.AnalyzeEBUR128Activity, activities.AnalyzeEBUR128Params{
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

func SetUploadedBy(ctx workflow.Context, assetID string, uploadedBy string) error {
	return wfutils.SetVidispineMeta(ctx, assetID, vscommon.FieldUploadedBy.Value, uploadedBy)
}

func SetUploadJobID(ctx workflow.Context, assetID string, jobID string) error {
	return wfutils.SetVidispineMeta(ctx, assetID, vscommon.FieldUploadJob.Value, jobID)
}

func addMetaTags(ctx workflow.Context, assetID string, metadata *ingest.Metadata) error {
	err := SetUploadedBy(ctx, assetID, metadata.JobProperty.SenderEmail)
	if err != nil {
		return err
	}

	err = SetUploadJobID(ctx, assetID, strconv.Itoa(metadata.JobProperty.JobID))
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

	program := ""
	if metadata.JobProperty.ProgramID != "" {
		// let workflow panic if the format is invalid?
		program = strings.Split(metadata.JobProperty.ProgramID, " - ")[1]
		if program != "" {
			err = wfutils.SetVidispineMeta(ctx, assetID, vscommon.FieldProgram.Value, program)
			if err != nil {
				return err
			}
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
		title := metadata.JobProperty.EpisodeTitle

		if program != "" {
			title = fmt.Sprintf("%s | %s", program, title)
		}

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
