package main

import (
	"github.com/bcc-code/bccm-flows/workflows"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.temporal.io/sdk/client"
)

func getParamFromCtx(ctx *gin.Context, key string) string {
	return ctx.DefaultPostForm(key, ctx.DefaultQuery(key, ""))
}

func triggerHandler(ctx *gin.Context) {
	job := ctx.Param("job")

	wfClient, err := client.Dial(client.Options{
		HostPort:  os.Getenv("TEMPORAL_HOST_PORT"),
		Namespace: os.Getenv("TEMPORAL_NAMESPACE"),
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	defer wfClient.Close()

	queue := os.Getenv("QUEUE")
	if queue == "" {
		queue = "worker"
	}
	vxID := getParamFromCtx(ctx, "vxID")
	workflowOptions := client.StartWorkflowOptions{
		ID:        uuid.NewString(),
		TaskQueue: queue,
		SearchAttributes: map[string]any{
			"VXID": vxID,
		},
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
	}

	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}
	ctx.JSON(http.StatusOK, res)
}

func main() {
	r := gin.Default()

	r.POST("/trigger/:job", triggerHandler)
	r.GET("/trigger/:job", triggerHandler)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080" // Default port if not specified
	}

	_ = r.Run(":" + port)
}
