package main

import (
	"net/http"
	"strings"

	"github.com/bcc-code/bcc-media-flows/workflows/export"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.temporal.io/sdk/client"
)

type BulkShortsExportParams struct {
	CollectionVXID string
}

func (s *TriggerServer) bulkShortsExportGET(ctx *gin.Context) {
	ctx.HTML(http.StatusOK, "bulk-shorts-export.gohtml", gin.H{
		"Title": "Bulk Shorts Export",
	})
}

func (s *TriggerServer) bulkShortsExportPOST(ctx *gin.Context) {
	collectionVXID := strings.TrimSpace(ctx.PostForm("collectionVXID"))
	// Remove all whitespace
	collectionVXID = strings.ReplaceAll(collectionVXID, " ", "")

	if !strings.HasPrefix(collectionVXID, "VX-") || len(collectionVXID) <= 3 {
		ctx.HTML(http.StatusBadRequest, "bulk-shorts-export.gohtml", gin.H{
			"Title":   "Bulk Shorts Export",
			"Error":   "Collection VXID must be in format VX-<NUMBERS>",
			"Entered": collectionVXID,
		})
		return
	}

	input := export.BulkExportShortsInput{CollectionVXID: collectionVXID}

	queue := getQueue()
	workflowOptions := client.StartWorkflowOptions{
		ID:        collectionVXID + "-bulk-shorts-" + uuid.NewString(),
		TaskQueue: queue,
	}

	res, err := s.wfClient.ExecuteWorkflow(ctx, workflowOptions, export.BulkExportShorts, input)
	if err != nil {
		renderErrorPage(ctx, http.StatusInternalServerError, err)
		return
	}

	ctx.HTML(http.StatusOK, "bulk-shorts-export.gohtml", gin.H{
		"Title":      "Bulk Shorts Export",
		"Success":    true,
		"WorkflowID": res.GetID(),
	})
}
