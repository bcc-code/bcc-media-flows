package main

import (
	ingestworkflows "github.com/bcc-code/bcc-media-flows/workflows/ingest"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/google/uuid"
	"go.temporal.io/sdk/client"
	"net/http"
)

func (s *TriggerServer) ingestFixGET(c *gin.Context) {
	c.HTML(http.StatusOK, "ingest-fix.gohtml", nil)
}

type mu1mu2ExtractForm struct {
	VX1ID string `form:"vx1" binding:"required"`
	VX2ID string `form:"vx2" binding:"required"`
}

func (s *TriggerServer) mu1mu2ExtractPOST(c *gin.Context) {
	var form mu1mu2ExtractForm
	err := c.ShouldBindWith(&form, binding.Form)
	if err != nil {
		renderErrorPage(c, http.StatusInternalServerError, err)
		return
	}

	queue := getQueue()
	workflowOptions := client.StartWorkflowOptions{
		ID:        uuid.NewString(),
		TaskQueue: queue,
	}

	_, err = s.wfClient.ExecuteWorkflow(c, workflowOptions, ingestworkflows.ExtractAudioFromMU1MU2, ingestworkflows.ExtractAudioFromMU1MU2Input{
		MU1ID: form.VX1ID,
		MU2ID: form.VX2ID,
	})
}
