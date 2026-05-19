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

	fmt.Printf("watcher event: path=%q size=%d updatedAt=%s\n", result.Path, result.Size, result.UpdatedAt.Format(time.RFC3339))

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
	rawImportFileCatalystDelivery2 := strings.HasPrefix(result.Path, "/mnt/filecatalyst/delivery2/RawMaterial/")
	fileboxSimpleUpload := strings.HasPrefix(result.Path, "/mnt/filecatalyst/delivery2/simple")

	var branch string
	if xmlPath {
		branch = "ingest"
		err = doIngest(ctx, result.Path)
	} else if multitrackPath {
		branch = "multitrack"
		err = doMultitrackCopy(ctx, result.Path)
	} else if growingPath {
		branch = "growing"
		err = doGrowingFile(ctx, result.Path)
	} else if rawImportIsilon || rawImportFileCatalyst || rawImportFileCatalystDelivery2 {
		branch = "raw-import"
		err = doRawImport(ctx, result.Path)
	} else if fileboxSimpleUpload {
		branch = "simple-copy"
		err = doSimpleCopy(ctx, result.Path)
	} else {
		branch = "transcode"
		err = doTranscode(ctx, result.Path)
	}
	fmt.Printf("watcher dispatched: path=%q branch=%s\n", result.Path, branch)

	if err != nil {
		fmt.Println(err.Error())
		ctx.String(500, err.Error())
		return
	}

	ctx.Status(200)
}

func doSimpleCopy(ctx context.Context, path string) error {
	c, err := getClient()
	if err != nil {
		return err
	}

	now := time.Now()
	destination := filepath.Join(
		"/mnt/isilon/Input/FromDelivery",
		now.Format("2006/01/02"),
		filepath.Base(path),
	)

	workflowOptions := client.StartWorkflowOptions{
		ID:        uuid.NewString(),
		TaskQueue: environment.GetWorkerQueue(),
	}

	_, err = c.ExecuteWorkflow(ctx, workflowOptions, miscworkflows.CopyFile, miscworkflows.CopyFileInput{
		Source:      path,
		Destination: destination,
	})

	return err
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

	// Use the fixed LIVE-INGEST workflow ID
	workflowOptions := client.StartWorkflowOptions{
		ID:        "LIVE-INGEST", // Fixed ID for the incremental workflow
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
