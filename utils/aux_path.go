package utils

import (
	"fmt"
	"time"

	"go.temporal.io/sdk/workflow"
)

var BaseDestinationPath = GetIsilonPrefix() + "/Production/aux"
var BaseRawDestinationPath = GetIsilonPrefix() + "/Production/raw"
var BaseTempPath = GetIsilonPrefix() + "/system/tmp"

// GetWorkflowOutputFolder retrieves the path and creates necessary folders for the workflow to use as an output.
func GetWorkflowOutputFolder(ctx workflow.Context) string {
	info := workflow.GetInfo(ctx)

	date := time.Now()

	return fmt.Sprintf("%s/%04d/%02d/%02d/%s", BaseDestinationPath, date.Year(), date.Month(), date.Day(), info.OriginalRunID)
}

// GetWorkflowRawOutputFolder retrieves the path and creates necessary folders for the workflow to use as an output.
func GetWorkflowRawOutputFolder(ctx workflow.Context) string {
	info := workflow.GetInfo(ctx)

	date := time.Now()

	return fmt.Sprintf("%s/%04d/%02d/%02d/%s", BaseRawDestinationPath, date.Year(), date.Month(), date.Day(), info.OriginalRunID)
}

// GetWorkflowTempFolder retrieves the path and creates necessary folders for the workflow to use as an output.
func GetWorkflowTempFolder(ctx workflow.Context) string {
	info := workflow.GetInfo(ctx)

	return fmt.Sprintf("%s/workflows/%s", BaseTempPath, info.OriginalRunID)
}
