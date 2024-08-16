package main

import (
	_ "embed"
	"fmt"
	"github.com/bcc-code/bcc-media-flows/utils"
	"net/http"
	"os"
	"reflect"
	"runtime"
	"strconv"

	"strings"

	"github.com/bcc-code/bcc-media-flows/environment"
	ingestworkflows "github.com/bcc-code/bcc-media-flows/workflows/ingest"
	miscworkflows "github.com/bcc-code/bcc-media-flows/workflows/misc"
	"github.com/gin-contrib/cors"

	"github.com/bcc-code/bcc-media-flows/workflows/export"

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
		res, err = wfClient.ExecuteWorkflow(ctx, workflowOptions, miscworkflows.TranscribeVX, miscworkflows.TranscribeVXInput{
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
		res, err = wfClient.ExecuteWorkflow(ctx, workflowOptions, miscworkflows.TranscribeFile, miscworkflows.TranscribeFileInput{
			Language:        language,
			DestinationPath: getParamFromCtx(ctx, "destinationPath"),
			File:            getParamFromCtx(ctx, "file"),
		})
	case "TranscodePreviewVX":
		if vxID == "" {
			ctx.Status(http.StatusBadRequest)
			return
		}
		res, err = wfClient.ExecuteWorkflow(ctx, workflowOptions, miscworkflows.TranscodePreviewVX, miscworkflows.TranscodePreviewVXInput{
			VXID: vxID,
		})
	case "TranscodePreviewFile":
		file := getParamFromCtx(ctx, "file")
		if file == "" {
			ctx.Status(http.StatusBadRequest)
			return
		}
		res, err = wfClient.ExecuteWorkflow(ctx, workflowOptions, miscworkflows.TranscodePreviewFile, miscworkflows.TranscodePreviewFileInput{
			FilePath: file,
		})
	case "ExportTimedMetadata":
		if vxID == "" {
			ctx.Status(http.StatusBadRequest)
			return
		}
		res, err = wfClient.ExecuteWorkflow(ctx, workflowOptions, export.ExportTimedMetadata, export.ExportTimedMetadataParams{
			VXID: vxID,
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

		resolutionsString := getParamFromCtx(ctx, "resolutions")
		var resolutions []utils.Resolution
		if resolutionsString != "" {
			for _, r := range strings.Split(resolutionsString, ",") {
				var width, height int
				_, err := fmt.Sscanf(r, "%dx%d", &width, &height)
				if err != nil {
					ctx.Status(http.StatusBadRequest)
					return
				}
				resolutions = append(resolutions, utils.Resolution{
					Width:  width,
					Height: height,
					File:   false,
				})
			}
		}

		res, err = wfClient.ExecuteWorkflow(ctx, workflowOptions, export.VXExport, export.VXExportParams{
			VXID:          vxID,
			WithChapters:  getParamFromCtx(ctx, "withChapters") == "true",
			WatermarkPath: getParamFromCtx(ctx, "watermarkPath"),
			Destinations:  strings.Split(getParamFromCtx(ctx, "destinations"), ","),
			Languages:     languages,
			Resolutions:   resolutions,
		})
	case "ExecuteFFmpeg":
		var input struct {
			Arguments []string `json:"arguments"`
		}
		if err = ctx.BindJSON(&input); err != nil {
			ctx.Status(http.StatusBadRequest)
			return
		}
		res, err = wfClient.ExecuteWorkflow(ctx, workflowOptions, miscworkflows.ExecuteFFmpeg, miscworkflows.ExecuteFFmpegInput{
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
		res, err = wfClient.ExecuteWorkflow(ctx, workflowOptions, miscworkflows.ImportSubtitlesFromSubtrans, miscworkflows.ImportSubtitlesFromSubtransInput{
			VXID: vxID,
		})
	case "UpdateAssetRelations":
		if vxID == "" {
			ctx.Status(http.StatusBadRequest)
			return
		}
		res, err = wfClient.ExecuteWorkflow(ctx, workflowOptions, miscworkflows.UpdateAssetRelations, miscworkflows.UpdateAssetRelationsParams{
			AssetID: vxID,
		})
	case "NormalizeAudio":
		target, err := strconv.ParseFloat(getParamFromCtx(ctx, "targetLUFS"), 64)
		if err != nil {
			_ = ctx.AbortWithError(http.StatusBadRequest, err)
			return
		}

		res, err = wfClient.ExecuteWorkflow(ctx, workflowOptions, miscworkflows.NormalizeAudioLevelWorkflow, miscworkflows.NormalizeAudioParams{
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
	r.Use(cors.Default())

	r.POST("/trigger/:job", triggerHandler)
	r.GET("/trigger/:job", triggerHandler)

	r.POST("/watchers", watchersHandler)

	r.GET("/trigger", func(ctx *gin.Context) {
		ctx.Writer.WriteString(html)
	})

	r.GET("/schemas", getWorkflowSchemas)
	r.POST("/trigger-dynamic", triggerDynamicHandler)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080" // Default port if not specified
	}

	_ = r.Run(":" + port)
}

func getFunctionName(i interface{}) (name string, isMethod bool) {
	if fullName, ok := i.(string); ok {
		return fullName, false
	}
	fullName := runtime.FuncForPC(reflect.ValueOf(i).Pointer()).Name()
	// Full function name that has a struct pointer receiver has the following format
	// <prefix>.(*<type>).<function>
	isMethod = strings.ContainsAny(fullName, "*")
	elements := strings.Split(fullName, ".")
	shortName := elements[len(elements)-1]
	// This allows to call activities by method pointer
	// Compiler adds -fm suffix to a function name which has a receiver
	// Note that this works even if struct pointer used to get the function is nil
	// It is possible because nil receivers are allowed.
	// For example:
	// var a *Activities
	// ExecuteActivity(ctx, a.Foo)
	// will call this function which is going to return "Foo"
	return strings.TrimSuffix(shortName, "-fm"), isMethod
}
