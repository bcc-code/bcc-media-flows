package ingestworkflows

import (
	"fmt"
	"github.com/bcc-code/bccm-flows/common"
	"github.com/bcc-code/bccm-flows/utils/wfutils"
	"go.temporal.io/sdk/workflow"
	"gopkg.in/guregu/null.v4"
	"path/filepath"
)

type VBMasterParams struct {
	Job common.IngestJob

	Directory string

	ProgramID      string
	ProgramQueueID null.String

	Filename string
	Language null.String

	Tags             []string
	PersonsAppearing []string
}

type VBMasterResult struct{}

func VBMaster(ctx workflow.Context, params VBMasterParams) (*VBMasterResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting VBMaster workflow")

	filename := params.ProgramID
	if params.ProgramQueueID.Valid {
		filename += "_" + params.ProgramQueueID.String
	}
	filename += "_" + params.Filename

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

	result, err := importTag(ctx, "original", file, filename)
	if err != nil {
		return nil, err
	}

	err = wfutils.WaitForVidispineJob(ctx, result.ImportJobID)
	if err != nil {
		return nil, err
	}

	return nil, nil
}
