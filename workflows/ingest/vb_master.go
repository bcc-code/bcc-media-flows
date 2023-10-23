package ingest

import (
	"go.temporal.io/sdk/workflow"
	"gopkg.in/guregu/null.v4"
)

type VBMasterParams struct {
	ProgramID      string
	ProgramQueueID null.String

	Filename string
	Language null.String

	Tags             []string
	PersonsAppearing []string
}

type VBMasterResult struct{}

func VBMaster(ctx workflow.Context, params VBMasterParams) (*VBMasterResult, error) {
	return nil, nil
}
