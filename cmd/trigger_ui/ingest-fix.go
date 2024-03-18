package main

import (
	"net/http"
	"strconv"

	ingestworkflows "github.com/bcc-code/bcc-media-flows/workflows/ingest"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/google/uuid"
	"go.temporal.io/sdk/client"
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

	var wfID string
	workflowOptions.ID = uuid.NewString()
	_, err = s.wfClient.ExecuteWorkflow(c, workflowOptions, ingestworkflows.ExtractAudioFromMU1MU2, ingestworkflows.ExtractAudioFromMU1MU2Input{
		MU1ID: form.VX1ID,
		MU2ID: form.VX2ID,
	})

	if err != nil {
		renderErrorPage(c, http.StatusInternalServerError, err)
		return
	}

	c.HTML(http.StatusOK, "success.gohtml", gin.H{
		"WorkflowID": wfID,
		"Title":      "Extract audio from MU1 and MU2",
	})
}

func (s *TriggerServer) ingestSyncFixGET(c *gin.Context) {
	c.HTML(http.StatusOK, "ingest-sync-fix.gohtml", nil)
}

type ingestSyncFixForm struct {
	VXID       string `form:"vxid" binding:"required"`
	Adjustment string `form:"adjustment" binding:"required"`
}

func (s *TriggerServer) ingestSyncFixPOST(c *gin.Context) {
	var form ingestSyncFixForm
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

	adjustment, err := strconv.ParseInt(form.Adjustment, 10, 64)
	if err != nil {
		renderErrorPage(c, http.StatusInternalServerError, err)
		return
	}

	_, err = s.wfClient.ExecuteWorkflow(c, workflowOptions, ingestworkflows.IngestSyncFix, ingestworkflows.IngestSyncFixParams{
		VXID:       form.VXID,
		Adjustment: int(adjustment),
	})

	if err != nil {
		renderErrorPage(c, http.StatusInternalServerError, err)
		return
	}

	c.HTML(http.StatusOK, "success.gohtml", gin.H{
		"WorkflowID": workflowOptions.ID,
		"Title":      "Adjusting audio sync",
	})
}
