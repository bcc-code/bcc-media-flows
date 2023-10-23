package ingestworkflows

import (
	"github.com/bcc-code/bccm-flows/common"
	"go.temporal.io/sdk/workflow"
	"gopkg.in/guregu/null.v4"
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

	return nil, nil
}
