package main

import (
	"net/http"
	"os"

	"github.com/bcc-code/bccm-flows/services/vidispine/vsapi"
	"github.com/bcc-code/bccm-flows/services/vidispine/vscommon"
	"github.com/bcc-code/bccm-flows/workflows/vb_export"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.temporal.io/sdk/client"
)

type VBTriggerGETParams struct {
	Title        string
	Destinations []string
}

func (s *TriggerServer) VBTriggerHandlerGET(c *gin.Context) {
	vxID := c.Query("id")
	meta, err := s.vidispine.GetMetadata(vxID)
	if err != nil {
		renderErrorPage(c, http.StatusInternalServerError, err)
		return
	}

	clips := meta.SplitByClips()
	title := clips[vsapi.OriginalClip].Get(vscommon.FieldTitle, "")

	c.HTML(http.StatusOK, "vb-export.gohtml", VBTriggerGETParams{
		Title:        title,
		Destinations: vb_export.VBExportDestinations.Values(),
	})
}

func (s *TriggerServer) VBTriggerHandlerPOST(c *gin.Context) {
	vxID := c.Query("id")

	queue := getQueue()
	workflowOptions := client.StartWorkflowOptions{
		TaskQueue: queue,
	}

	if os.Getenv("DEBUG") == "" {
		workflowOptions.SearchAttributes = map[string]any{
			"CustomStringField": vxID,
		}
	}

	params := vb_export.VBExportParams{
		VXID:         vxID,
		Destinations: c.PostFormArray("destinations[]"),
	}

	var wfID string
	workflowOptions.ID = uuid.NewString()
	res, err := s.wfClient.ExecuteWorkflow(c, workflowOptions, vb_export.VBExport, params)

	if err != nil {
		renderErrorPage(c, http.StatusInternalServerError, err)
		return
	}

	wfID = res.GetID()

	meta, err := s.vidispine.GetMetadata(vxID)
	if err != nil {
		renderErrorPage(c, http.StatusInternalServerError, err)
		return
	}

	c.HTML(http.StatusOK, "success.gohtml", gin.H{
		"WorkflowID": wfID,
		"Title":      meta.Get(vscommon.FieldTitle, ""),
	})
}
