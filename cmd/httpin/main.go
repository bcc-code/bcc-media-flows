package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"

	"github.com/bcc-code/bcc-media-flows/environment"
	"github.com/bcc-code/bcc-media-flows/workflows/ingest"
	"github.com/bcc-code/bcc-media-flows/workflows/webhooks"
	"github.com/gin-gonic/gin/binding"

	"strings"

	"github.com/bcc-code/bcc-media-flows/workflows/export"

	"github.com/bcc-code/bcc-media-flows/workflows"

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
		queue = environment.QueueWorker
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
	case "TranscribeFile":
		language := getParamFromCtx(ctx, "language")
		destinationPath := getParamFromCtx(ctx, "destinationPath")
		file := getParamFromCtx(ctx, "file")

		if language == "" || destinationPath == "" || file == "" {
			ctx.Status(http.StatusBadRequest)
			return
		}
		res, err = wfClient.ExecuteWorkflow(ctx, workflowOptions, workflows.TranscribeFile, workflows.TranscribeFileInput{
			Language:        language,
			DestinationPath: getParamFromCtx(ctx, "destinationPath"),
			File:            getParamFromCtx(ctx, "file"),
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

		languagesString := getParamFromCtx(ctx, "languages")
		var languages []string
		if languagesString != "" {
			languages = strings.Split(languagesString, ",")
		}

		res, err = wfClient.ExecuteWorkflow(ctx, workflowOptions, export.VXExport, export.VXExportParams{
			VXID:          vxID,
			WithChapters:  getParamFromCtx(ctx, "withChapters") == "true",
			WatermarkPath: getParamFromCtx(ctx, "watermarkPath"),
			Destinations:  strings.Split(getParamFromCtx(ctx, "destinations"), ","),
			Languages:     languages,
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
	case "AssetIngest":
		xmlPath := getParamFromCtx(ctx, "xmlPath")
		if xmlPath == "" {
			ctx.Status(http.StatusBadRequest)
			return
		}
		res, err = wfClient.ExecuteWorkflow(ctx, workflowOptions, ingestworkflows.Asset, ingestworkflows.AssetParams{
			XMLPath: xmlPath,
		})
	case "ImportSubtitlesFromSubtrans":
		if vxID == "" {
			ctx.Status(http.StatusBadRequest)
			return
		}
		res, err = wfClient.ExecuteWorkflow(ctx, workflowOptions, workflows.ImportSubtitlesFromSubtrans, workflows.ImportSubtitlesFromSubtransInput{
			VXID: vxID,
		})
	case "NormalizeAudio":
		target, err := strconv.ParseFloat(getParamFromCtx(ctx, "targetLUFS"), 64)
		if err != nil {
			_ = ctx.AbortWithError(http.StatusBadRequest, err)
			return
		}

		res, err = wfClient.ExecuteWorkflow(ctx, workflowOptions, workflows.NormalizeAudioLevelWorkflow, workflows.NormalizeAudioParams{
			FilePath:              getParamFromCtx(ctx, "file"),
			TargetLUFS:            target,
			PerformOutputAnalysis: true,
		})
	case "IncrementalIngest":
		path := getParamFromCtx(ctx, "path")
		if path == "" {
			ctx.Status(http.StatusBadRequest)
			return
		}
		res, err = wfClient.ExecuteWorkflow(ctx, workflowOptions, ingestworkflows.Incremental, ingestworkflows.IncrementalParams{
			Path: path,
		})
	case "WebHook":
		var rawMessage json.RawMessage
		if err = ctx.ShouldBindBodyWith(&rawMessage, binding.JSON); err != nil {
			ctx.Status(http.StatusBadRequest)
			return
		}
		res, err = wfClient.ExecuteWorkflow(ctx, workflowOptions, webhooks.WebHook, webhooks.WebHookInput{
			Type:       getParamFromCtx(ctx, "type"),
			Parameters: rawMessage,
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
