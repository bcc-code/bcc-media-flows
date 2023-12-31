package main

import (
	"context"
	"fmt"
	"github.com/bcc-code/bcc-media-flows/environment"
	"github.com/bcc-code/bcc-media-flows/workflows"
	"github.com/bcc-code/bcc-media-flows/workflows/ingest"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.temporal.io/sdk/client"
	"os"
	"path/filepath"
	"regexp"
	"time"
)

var TranscodeRootPath = os.Getenv("TRANSCODE_ROOT_PATH")

type watcherResult struct {
	Name      string    `json:"name"`
	Path      string    `json:"path"`
	UpdatedAt time.Time `json:"updatedAt"`
	Size      int64     `json:"size"`
}

func watchersHandler(ctx *gin.Context) {
	var result watcherResult
	err := ctx.BindJSON(&result)
	if err != nil {
		fmt.Println(err.Error())
		ctx.String(400, err.Error())
		return
	}

	xmlPath, err := filepath.Match("/mnt/dmzshare/workflow/xml/*", result.Path)
	if err != nil {
		fmt.Println(err.Error())
		ctx.String(500, err.Error())
		return
	}
	if xmlPath {
		err = doIngest(ctx, result.Path)
	} else {
		err = doTranscode(ctx, result.Path)
	}

	if err != nil {
		fmt.Println(err.Error())
		ctx.String(500, err.Error())
		return
	}

	ctx.Status(200)
}

var exp = regexp.MustCompile(fmt.Sprintf("(?:%s/)(?P<encoding>[\\w-]*)(?:/in/)", TranscodeRootPath))

func doTranscode(ctx context.Context, path string) error {
	match := exp.MatchString(path)
	if !match {
		return fmt.Errorf("%s not matched", path)
	}

	c, err := getClient()
	if err != nil {
		return err
	}

	matches := exp.FindStringSubmatch(path)
	t := matches[1]

	workflowOptions := client.StartWorkflowOptions{
		ID:        uuid.NewString(),
		TaskQueue: environment.GetWorkerQueue(),
	}

	_, err = c.ExecuteWorkflow(ctx, workflowOptions, workflows.WatchFolderTranscode, workflows.WatchFolderTranscodeInput{
		Path:       path,
		FolderName: t,
	})
	return err
}

func doIngest(ctx context.Context, path string) error {
	c, err := getClient()
	if err != nil {
		return err
	}

	workflowOptions := client.StartWorkflowOptions{
		ID:        uuid.NewString(),
		TaskQueue: environment.GetWorkerQueue(),
	}

	_, err = c.ExecuteWorkflow(ctx, workflowOptions, ingestworkflows.Asset, ingestworkflows.AssetParams{
		XMLPath: path,
	})
	return err
}
