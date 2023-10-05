package main

import (
	_ "embed"
	"fmt"
	"net/http"
	"os"
	"strconv"

	"strings"

	"github.com/bcc-code/bccm-flows/workflows/export"

	"github.com/bcc-code/bccm-flows/common"
	"github.com/bcc-code/bccm-flows/workflows"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.temporal.io/sdk/client"
)

func getParamFromCtx(ctx *gin.Context, key string) string {
	return ctx.DefaultPostForm(key, ctx.DefaultQuery(key, ""))
}

func getClient() (client.Client, error) {
	return client.Dial(client.Options{
		HostPort:  os.Getenv("TEMPORAL_HOST_PORT"),
		Namespace: os.Getenv("TEMPORAL_NAMESPACE"),
	})
}

func getQueue() string {
	queue := os.Getenv("QUEUE")
	if queue == "" {
		queue = common.QueueWorker
	}
	return queue
}

func triggerHandler(ctx *gin.Context) {
	job := ctx.Param("job")

	wfClient, err := getClient()
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	defer wfClient.Close()

	queue := getQueue()
	vxID := getParamFromCtx(ctx, "vxID")
	workflowOptions := client.StartWorkflowOptions{
		ID:        uuid.NewString(),
		TaskQueue: queue,
	}

	if os.Getenv("DEBUG") == "" {
		workflowOptions.SearchAttributes = map[string]any{
			"CustomStringField": vxID,
		}
	}

	var res client.WorkflowRun

	switch job {
	case "TranscribeVX":
		language := getParamFromCtx(ctx, "language")
		if vxID == "" || language == "" {
			ctx.Status(http.StatusBadRequest)
			return
		}
		res, err = wfClient.ExecuteWorkflow(ctx, workflowOptions, workflows.TranscribeVX, workflows.TranscribeVXInput{
			Language: language,
			VXID:     vxID,
		})
	case "TranscodePreviewVX":
		if vxID == "" {
			ctx.Status(http.StatusBadRequest)
			return
		}
		res, err = wfClient.ExecuteWorkflow(ctx, workflowOptions, workflows.TranscodePreviewVX, workflows.TranscodePreviewVXInput{
			VXID: vxID,
		})
	case "TranscodePreviewFile":
		file := getParamFromCtx(ctx, "file")
		if file == "" {
			ctx.Status(http.StatusBadRequest)
			return
		}
		res, err = wfClient.ExecuteWorkflow(ctx, workflowOptions, workflows.TranscodePreviewFile, workflows.TranscodePreviewFileInput{
			FilePath: file,
		})
	case "ExportAssetVX":
		if vxID == "" {
			ctx.Status(http.StatusBadRequest)
			return
		}

		res, err = wfClient.ExecuteWorkflow(ctx, workflowOptions, export.VXExport, export.VXExportParams{
			VXID:          vxID,
			WithFiles:     getParamFromCtx(ctx, "withFiles") == "true",
			WithChapters:  getParamFromCtx(ctx, "withChapters") == "true",
			WatermarkPath: getParamFromCtx(ctx, "watermarkPath"),
			Destinations:  strings.Split(getParamFromCtx(ctx, "destinations"), ","),
		})
	case "ExecuteFFmpeg":
		var input struct {
			Arguments []string `json:"arguments"`
		}
		if err = ctx.BindJSON(&input); err != nil {
			ctx.Status(http.StatusBadRequest)
			return
		}
		res, err = wfClient.ExecuteWorkflow(ctx, workflowOptions, workflows.ExecuteFFmpeg, workflows.ExecuteFFmpegInput{
			Arguments: input.Arguments,
		})
	case "ImportSubtitlesFromSubtrans":
		if vxID == "" {
			ctx.Status(http.StatusBadRequest)
			return
		}
		res, err = wfClient.ExecuteWorkflow(ctx, workflowOptions, workflows.ImportSubtitlesFromSubtrans, workflows.ImportSubtitlesFromSubtransInput{
			VXId: vxID,
		})
	case "NormalizeAudio":
		target, err := strconv.ParseFloat(getParamFromCtx(ctx, "targetLUFS"), 64)
		if err != nil {
			ctx.AbortWithError(http.StatusBadRequest, err)
			return
		}

		res, err = wfClient.ExecuteWorkflow(ctx, workflowOptions, workflows.NormalizeAudioLevelWorkflow, workflows.NormalizeAudioParams{
			FilePath:              getParamFromCtx(ctx, "file"),
			TargetLUFS:            target,
			PerformOutputAnalysis: true,
		})
	}

	if err != nil {
		fmt.Print(err)
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}
	ctx.JSON(http.StatusOK, res)
}

//go:embed trigger.html
var html string

func main() {
	r := gin.Default()

	r.POST("/trigger/:job", triggerHandler)
	r.GET("/trigger/:job", triggerHandler)

	r.POST("/watchers", watchersHandler)

	r.GET("/trigger", func(ctx *gin.Context) {
		ctx.Writer.WriteString(html)
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080" // Default port if not specified
	}

	_ = r.Run(":" + port)
}
