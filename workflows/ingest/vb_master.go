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

	programID := params.Metadata.JobProperty.ProgramID
	if programID != "" {
		programID = strings.Split(programID, " ")[0]
	}

	filename := programID
	if params.Metadata.JobProperty.ProgramPost != "" {
		filename += "_" + params.Metadata.JobProperty.ProgramPost
	}
	filename += "_" + params.Metadata.JobProperty.ReceivedFilename

	if nonAlphanumeric.MatchString(filename) {
		return nil, fmt.Errorf("filename contains non-alphanumeric characters: %s", filename)
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

	sourceFile := files[0]

	if filepath.Ext(sourceFile) != filepath.Ext(filename) {
		filename += filepath.Ext(sourceFile)
	}

	// make sure the filename without extension is uppercase
	filename = strings.ToUpper(filename[:len(filename)-len(filepath.Ext(filename))]) + filepath.Ext(filename)

	//Production/masters/{date}/{wfID}/{filename}
	file := filepath.Join(outputDir, filename)
	err = wfutils.MoveFile(ctx, files[0], file)
	if err != nil {
		return nil, err
	}

	result, err := importFileAsTag(ctx, "original", file, filename)
	if err != nil {
		return nil, err
	}

	err = wfutils.SetVidispineMeta(ctx, result.AssetID, vscommon.FieldUploadedBy.Value, params.Metadata.JobProperty.SenderEmail)
	if err != nil {
		return nil, err
	}

	err = wfutils.SetVidispineMeta(ctx, result.AssetID, vscommon.FieldUploadJob.Value, strconv.Itoa(params.Metadata.JobProperty.JobID))
	if err != nil {
		return nil, err
	}

	if params.Metadata.JobProperty.PersonsAppearing != "" {
		for _, person := range strings.Split(params.Metadata.JobProperty.PersonsAppearing, ",") {
			person = strings.TrimSpace(person)
			if person == "" {
				continue
			}
			err = wfutils.AddVidispineMetaValue(ctx, result.AssetID, vscommon.FieldPersonsAppearing.Value, person)
			if err != nil {
				return nil, err
			}
		}
	}

	if params.Metadata.JobProperty.Tags != "" {
		for _, tag := range strings.Split(params.Metadata.JobProperty.Tags, ",") {
			tag = strings.TrimSpace(tag)
			if tag == "" {
				continue
			}
			err = wfutils.AddVidispineMetaValue(ctx, result.AssetID, vscommon.FieldSource.Value, tag)
			if err != nil {
				return nil, err
			}
		}
	}

	err = wfutils.WaitForVidispineJob(ctx, result.ImportJobID)
	if err != nil {
		return nil, err
	}

	path, err := utils.ParsePath(file)
	if err != nil {
		return nil, err
	}

	err = wfutils.ExecuteWithQueue(ctx, batonactivities.QC, batonactivities.QCParams{
		Path: path,
		Plan: baton.TestPlanMXF,
	}).Get(ctx, nil)
	if err != nil {
		return nil, err
	}

	return nil, nil
}
