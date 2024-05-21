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

func (s *TriggerServer) ingestFixGET(ctx *gin.Context) {
	ctx.HTML(http.StatusOK, "ingest-fix.gohtml", nil)
}

type mu1mu2ExtractForm struct {
	VX1ID string `form:"vx1" binding:"required"`
	VX2ID string `form:"vx2" binding:"required"`
}

func (s *TriggerServer) mu1mu2ExtractPOST(ctx *gin.Context) {
	var form mu1mu2ExtractForm
	err := ctx.ShouldBindWith(&form, binding.Form)
	if err != nil {
		renderErrorPage(ctx, http.StatusInternalServerError, err)
		return
	}

	queue := getQueue()
	workflowOptions := client.StartWorkflowOptions{
		ID:        uuid.NewString(),
		TaskQueue: queue,
	}

	var wfID string
	workflowOptions.ID = uuid.NewString()
	_, err = s.wfClient.ExecuteWorkflow(ctx, workflowOptions, ingestworkflows.ExtractAudioFromMU1MU2, ingestworkflows.ExtractAudioFromMU1MU2Input{
		MU1ID: form.VX1ID,
		MU2ID: form.VX2ID,
	})

	if err != nil {
		renderErrorPage(ctx, http.StatusInternalServerError, err)
		return
	}

	ctx.HTML(http.StatusOK, "success.gohtml", gin.H{
		"WorkflowID": wfID,
		"Title":      "Extract audio from MU1 and MU2",
	})
}

func (s *TriggerServer) ingestSyncFixGET(ctx *gin.Context) {
	ctx.HTML(http.StatusOK, "ingest-sync-fix.gohtml", nil)
}

type ingestSyncFixForm struct {
	VXID       string `form:"vxid" binding:"required"`
	Adjustment string `form:"adjustment" binding:"required"`
}

func (s *TriggerServer) ingestSyncFixPOST(ctx *gin.Context) {
	var form ingestSyncFixForm
	err := ctx.ShouldBindWith(&form, binding.Form)
	if err != nil {
		renderErrorPage(ctx, http.StatusInternalServerError, err)
		return
	}

	queue := getQueue()
	workflowOptions := client.StartWorkflowOptions{
		ID:        uuid.NewString(),
		TaskQueue: queue,
	}

	adjustment, err := strconv.ParseInt(form.Adjustment, 10, 64)
	if err != nil {
		renderErrorPage(ctx, http.StatusInternalServerError, err)
		return
	}

	_, err = s.wfClient.ExecuteWorkflow(ctx, workflowOptions, ingestworkflows.IngestSyncFix, ingestworkflows.IngestSyncFixParams{
		VXID:       form.VXID,
		Adjustment: int(adjustment),
	})

	if err != nil {
		renderErrorPage(ctx, http.StatusInternalServerError, err)
		return
	}

	ctx.HTML(http.StatusOK, "success.gohtml", gin.H{
		"WorkflowID": workflowOptions.ID,
		"Title":      "Adjusting audio sync",
	})
}
