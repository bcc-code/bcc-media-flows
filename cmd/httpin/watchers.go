package main

import (
	"context"
	"fmt"
	"github.com/bcc-code/bccm-flows/utils"
	"github.com/bcc-code/bccm-flows/workflows"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.temporal.io/sdk/client"
	"os"
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
		print(err)
	}

	err = doTranscode(ctx, result.Path)
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
		TaskQueue: utils.GetWorkerQueue(),
	}

	_, err = c.ExecuteWorkflow(ctx, workflowOptions, workflows.WatchFolderTranscode, workflows.WatchFolderTranscodeInput{
		Path:       path,
		FolderName: t,
	})
	return err
}
