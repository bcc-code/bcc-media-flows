package ingestworkflows

import (
	"fmt"
	batonactivities "github.com/bcc-code/bccm-flows/activities/baton"
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

type VBMasterParams struct {
	Metadata *ingest.Metadata

	Directory string
}

type VBMasterResult struct{}

// regexp for making sure the filename does not contain non-alphanumeric characters
var nonAlphanumeric = regexp.MustCompile("[^a-zA-Z0-9_]")

func VBMaster(ctx workflow.Context, params VBMasterParams) (*VBMasterResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting VBMaster workflow")

	ctx = workflow.WithActivityOptions(ctx, wfutils.GetDefaultActivityOptions())

	filename, err := vbMasterFilename(params.Metadata)
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

	filename += filepath.Ext(files[0])

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

	err = wfutils.ExecuteWithQueue(ctx, batonactivities.QC, batonactivities.QCParams{
		Path: path,
		Plan: plan,
	}).Get(ctx, nil)
	if err != nil {
		return nil, err
	}

	return nil, nil
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
	return nil
}
