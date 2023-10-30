package ingestworkflows

import (
	"fmt"
	"github.com/bcc-code/bccm-flows/services/ingest"
	"github.com/bcc-code/bccm-flows/services/vidispine/vscommon"
	"github.com/bcc-code/bccm-flows/utils/wfutils"
	"go.temporal.io/sdk/workflow"
	"path/filepath"
	"strconv"
	"strings"
)

type VBMasterParams struct {
	Metadata *ingest.Metadata

	Directory string
}

type VBMasterResult struct{}

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

	outputDir, err := wfutils.GetWorkflowOutputFolder(ctx)
	if err != nil {
		return nil, err
	}

	//Production/aux/{date}/{wfID}/{filename}
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

	err = wfutils.WaitForVidispineJob(ctx, result.ImportJobID)
	if err != nil {
		return nil, err
	}

	return nil, nil
}
