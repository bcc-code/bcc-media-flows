package utils

import (
	"fmt"
	"go.temporal.io/sdk/workflow"
	"time"
)

const BaseDestinationPath = "/mnt/isilon/Production/aux"
const BaseTempPath = "/mnt/isilon/system/tmp"

// GetWorkflowOutputFolder retrieves the path and creates necessary folders for the workflow to use as an output.
func GetWorkflowOutputFolder(ctx workflow.Context) string {
	info := workflow.GetInfo(ctx)

	date := time.Now()

	return fmt.Sprintf("%s/%04d/%02d/%02d/%s", BaseDestinationPath, date.Year(), date.Month(), date.Day(), info.OriginalRunID)
}

// GetWorkflowTempFolder retrieves the path and creates necessary folders for the workflow to use as an output.
func GetWorkflowTempFolder(ctx workflow.Context) string {
	info := workflow.GetInfo(ctx)

	return fmt.Sprintf("%s/workflows/%s", BaseTempPath, info.OriginalRunID)
}
