package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/bcc-code/bcc-media-flows/environment"
	"github.com/bcc-code/bcc-media-flows/paths"
	ingestworkflows "github.com/bcc-code/bcc-media-flows/workflows/ingest"
	miscworkflows "github.com/bcc-code/bcc-media-flows/workflows/misc"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.temporal.io/sdk/client"
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

	xmlPath, err := filepath.Match("/mnt/filecatalyst/workflow/xml/*", result.Path)
	if err != nil {
		fmt.Println(err.Error())
		ctx.String(500, err.Error())
		return
	}

	// This needs to match any subfolder
	multitrackPath := strings.HasPrefix(result.Path, "/mnt/filecatalyst/multitrack/Ingest/tempFraBrunstad/")
	growingPath := strings.HasPrefix(result.Path, "/mnt/filecatalyst/ingestgrow/")
	rawImportIsilon := strings.HasPrefix(result.Path, "/mnt/isilon/Input/Rawmaterial/")
	rawImportFileCatalyst := strings.HasPrefix(result.Path, "/mnt/filecatalyst/Rawmaterial/")

	if xmlPath {
		err = doIngest(ctx, result.Path)
	} else if multitrackPath {
		err = doMultitrackCopy(ctx, result.Path)
	} else if growingPath {
		err = doGrowingFile(ctx, result.Path)
	} else if rawImportIsilon || rawImportFileCatalyst {
		err = doRawImport(ctx, result.Path)
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

func doMultitrackCopy(ctx context.Context, path string) error {
	c, err := getClient()
	if err != nil {
		return err
	}

	workflowOptions := client.StartWorkflowOptions{
		ID:        uuid.NewString(),
		TaskQueue: environment.GetWorkerQueue(),
	}

	_, err = c.ExecuteWorkflow(ctx, workflowOptions, miscworkflows.HandleMultitrackFile, miscworkflows.HandleMultitrackFileInput{
		Path: path,
	})

	return err
}

func doGrowingFile(ctx context.Context, path string) error {
	c, err := getClient()
	if err != nil {
		return err
	}

	workflowOptions := client.StartWorkflowOptions{
		ID:        uuid.NewString(),
		TaskQueue: environment.GetWorkerQueue(),
	}

	_, err = c.ExecuteWorkflow(ctx, workflowOptions, ingestworkflows.Incremental, ingestworkflows.IncrementalParams{
		Path: path,
	})

	return err
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

	_, err = c.ExecuteWorkflow(ctx, workflowOptions, miscworkflows.WatchFolderTranscode, miscworkflows.WatchFolderTranscodeInput{
		Path:       path,
		FolderName: t,
	})
	return err
}

func doRawImport(ctx context.Context, path string) error {
	c, err := getClient()
	if err != nil {
		return err
	}

	workflowOptions := client.StartWorkflowOptions{
		ID:        "RAWIMPORT-" + uuid.NewString(),
		TaskQueue: environment.GetWorkerQueue(),
	}

	parsedPath, err := paths.Parse(path)
	if err != nil {
		return err
	}

	_, err = c.ExecuteWorkflow(ctx, workflowOptions, ingestworkflows.RawMaterial, ingestworkflows.RawMaterialParams{
		FilesToIngest: paths.Files{parsedPath},
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
