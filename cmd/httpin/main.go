package main

import (
	"fmt"
	"github.com/bcc-code/bcc-media-flows/internal/bootstrap"
	"net/http"
	"strconv"

	"github.com/bcc-code/bcc-media-flows/utils"

	"strings"

	"github.com/bcc-code/bcc-media-flows/environment"
	wfutils "github.com/bcc-code/bcc-media-flows/utils/workflows"
	ingestworkflows "github.com/bcc-code/bcc-media-flows/workflows/ingest"
	miscworkflows "github.com/bcc-code/bcc-media-flows/workflows/misc"
	"github.com/gin-contrib/cors"

	"github.com/bcc-code/bcc-media-flows/workflows/export"

	"github.com/gin-gonic/gin"
	"go.temporal.io/sdk/client"
)

func getParamFromCtx(ctx *gin.Context, key string) string {
	return ctx.DefaultPostForm(key, ctx.DefaultQuery(key, ""))
}

func getTriggeredBy(ctx *gin.Context) string {
	triggeredBy := getParamFromCtx(ctx, "triggeredBy")
	if triggeredBy == "" {
		triggeredBy = "httpin"
	}
	return triggeredBy
}

// temporalClient is dialed once in main and shared by all handlers.
var temporalClient client.Client

func getClient() (client.Client, error) {
	return temporalClient, nil
}

func getQueue() string {
	return environment.GetQueue()
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

	queue := getQueue()
	vxID := getParamFromCtx(ctx, "vxID")
	workflowOptions := wfutils.NewWorkflowOptions(queue, vxID, getTriggeredBy(ctx))

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
	case "CreateThumbnailsVX":
		if vxID == "" {
			ctx.Status(http.StatusBadRequest)
			return
		}
		width, _ := strconv.Atoi(getParamFromCtx(ctx, "width"))
		height, _ := strconv.Atoi(getParamFromCtx(ctx, "height"))
		res, err = wfClient.ExecuteWorkflow(ctx, workflowOptions, miscworkflows.CreateThumbnailsVX, miscworkflows.CreateThumbnailsVXInput{
			VXID:   vxID,
			Width:  width,
			Height: height,
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
					IsFile: false,
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
	case "AssetIngestJSON":
		jsonPath := getParamFromCtx(ctx, "jsonPath")
		if jsonPath == "" {
			ctx.Status(http.StatusBadRequest)
			return
		}
		res, err = wfClient.ExecuteWorkflow(ctx, workflowOptions, ingestworkflows.AssetJSON, ingestworkflows.AssetJSONParams{
			JSONPath: jsonPath,
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
		// parseErr, not err: declaring a case-scoped err here shadowed the one the
		// check after the switch reads, so a failed ExecuteWorkflow below was reported
		// as 200 OK with a null body.
		target, parseErr := strconv.ParseFloat(getParamFromCtx(ctx, "targetLUFS"), 64)
		if parseErr != nil {
			_ = ctx.AbortWithError(http.StatusBadRequest, parseErr)
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
	default:
		// Without this, an unknown or renamed job name left res and err both nil and
		// fell through to the 200 below, so callers — including the FileCatalyst and
		// watcher integrations — read a typo as success.
		ctx.JSON(http.StatusNotFound, gin.H{
			"error": fmt.Sprintf("unknown job: %s", job),
		})
		return
	}

	if err != nil {
		fmt.Print(err)
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	// A case that returns without either starting a workflow or writing a response
	// would otherwise be reported as success. Cheaper to catch here than to rely on
	// every future case remembering.
	if res == nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("job %s did not start a workflow", job),
		})
		return
	}

	ctx.JSON(http.StatusOK, res)
}

func main() {
	bootstrap.LoadEnv()
	environment.Load()
	environment.WarnMissing(environment.RequiredByHTTPIn)

	var err error
	temporalClient, err = bootstrap.TemporalClient()
	if err != nil {
		panic(err)
	}
	defer temporalClient.Close()

	r := gin.Default()
	r.Use(cors.Default())

	r.POST("/trigger/:job", triggerHandler)
	r.GET("/trigger/:job", triggerHandler)

	r.POST("/watchers", watchersHandler)

	r.POST("/ingest/json", jsonIngestHandler)

	_ = bootstrap.Serve(r, "8080")
}
