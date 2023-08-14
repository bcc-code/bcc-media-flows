package main

import (
	"net/http"
	"os"

	"github.com/bcc-code/bccm-flows/workflows/transcribe"
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

	workflowOptions := client.StartWorkflowOptions{
		ID:        "generic-worker-" + uuid.NewString(),
		TaskQueue: "generic-worker",
	}

	// TODO: Ugly code, just a test
	if vxID != "" {

		transcribeInput := transcribe.TranscribeVXWorkflowInput{
			Language:        language,
			DestinationPath: destinationPath,
			VXID:            vxID,
		}

		res, err := wfClient.ExecuteWorkflow(c, workflowOptions, transcribe.TranscribeVXWorkflow, transcribeInput)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
			return
		}
		c.JSON(http.StatusOK, res)
		return
	}

	transcribeInput := transcribe.TranscribeFileWorkflowInput{
		Language:        language,
		File:            file,
		DestinationPath: destinationPath,
	}

	res, err := wfClient.ExecuteWorkflow(c, workflowOptions, transcribe.TranscribeWorkflow, transcribeInput)

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

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080" // Default port if not specified
	}

	r.Run(":" + port)
}
