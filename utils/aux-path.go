package utils

import (
	"fmt"
	"go.temporal.io/sdk/workflow"
	"os"
	"time"
)

const BaseDestinationPath = "/mnt/isilon/Production/aux"

func GetWorkflowOutputFolder(ctx workflow.Context) (string, error) {
	info := workflow.GetInfo(ctx)

	date := time.Now()

	destinationPath := fmt.Sprintf("%s/%04d/%02d/%02d/%s", BaseDestinationPath, date.Year(), date.Month(), date.Day(), info.OriginalRunID)

	err := os.MkdirAll(destinationPath, os.ModePerm)

	return destinationPath, err
}
