package ingest

import (
	"github.com/bcc-code/bccm-flows/activities"
	"github.com/bcc-code/bccm-flows/common"
	"github.com/bcc-code/bccm-flows/utils"
	"github.com/bcc-code/bccm-flows/utils/wfutils"
	"go.temporal.io/sdk/workflow"
	"gopkg.in/guregu/null.v4"
)

type VBMasterParams struct {
	Job common.IngestJob

	FilePath utils.Path

	ProgramID      string
	ProgramQueueID null.String

	Filename string
	Language null.String

	Tags             []string
	PersonsAppearing []string
}

type VBMasterResult struct{}

func VBMaster(ctx workflow.Context, params VBMasterParams) (*VBMasterResult, error) {
	filename := params.ProgramID
	if params.ProgramQueueID.Valid {
		filename += "_" + params.ProgramQueueID.String
	}
	filename += "_" + params.Filename

	dir, err := wfutils.GetWorkflowTempFolder(ctx)
	if err != nil {
		return nil, err
	}

	path, err := utils.ParsePath(dir)
	if err != nil {
		return nil, err
	}

	workflow.ExecuteActivity(ctx, activities.RcloneMoveFile, activities.RcloneMoveFileInput{
		Source:      params.FilePath,
		Destination: path,
	})

	return nil, nil
}
