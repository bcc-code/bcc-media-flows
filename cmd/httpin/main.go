package main

import (
	"github.com/bcc-code/bccm-flows/workflows"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.temporal.io/sdk/client"
)

func transcribeHandler(c *gin.Context) {
	language := c.DefaultPostForm("language", c.DefaultQuery("language", ""))
	file := c.DefaultPostForm("file", c.DefaultQuery("file", ""))
	destinationPath := c.DefaultPostForm("destinationPath", c.DefaultQuery("destinationPath", ""))
	vxID := c.DefaultPostForm("vxID", c.DefaultQuery("vxID", ""))

	wfClient, err := client.Dial(client.Options{
		HostPort:  os.Getenv("TEMPORAL_HOST_PORT"),
		Namespace: os.Getenv("TEMPORAL_NAMESPACE"),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	defer wfClient.Close()

	queue := os.Getenv("QUEUE")
	if queue == "" {
		queue = "worker"
	}
	workflowOptions := client.StartWorkflowOptions{
		ID:        "worker-" + uuid.NewString(),
		TaskQueue: queue,
	}

	// TODO: Ugly code, just a test
	if vxID != "" {

		transcribeInput := workflows.TranscribeVXInput{
			Language: language,
			VXID:     vxID,
		}

		res, err := wfClient.ExecuteWorkflow(c, workflowOptions, workflows.TranscribeVX, transcribeInput)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
			return
		}
		c.JSON(http.StatusOK, res)
		return
	}

	transcribeInput := workflows.TranscribeFileInput{
		Language:        language,
		File:            file,
		DestinationPath: destinationPath,
	}

	res, err := wfClient.ExecuteWorkflow(c, workflowOptions, workflows.TranscribeFile, transcribeInput)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, res)
}

func transcodeHandler(c *gin.Context) {
	file := c.DefaultPostForm("file", c.DefaultQuery("file", ""))

	wfClient, err := client.Dial(client.Options{
		HostPort:  os.Getenv("TEMPORAL_HOST_PORT"),
		Namespace: os.Getenv("TEMPORAL_NAMESPACE"),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	defer wfClient.Close()

	queue := os.Getenv("QUEUE")
	if queue == "" {
		queue = "worker"
	}
	workflowOptions := client.StartWorkflowOptions{
		ID:        "worker-" + uuid.NewString(),
		TaskQueue: queue,
	}

	transcodeInput := workflows.TranscodeFileInput{
		FilePath: file,
	}

	res, err := wfClient.ExecuteWorkflow(c, workflowOptions, workflows.TranscodeFile, transcodeInput)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, res)
}

func main() {
	r := gin.Default()

	r.GET("/transcribe", transcribeHandler)
	r.POST("/transcribe", transcribeHandler)

	r.GET("/transcode", transcodeHandler)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080" // Default port if not specified
	}

	r.Run(":" + port)
}
