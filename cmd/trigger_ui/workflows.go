package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bcc-code/bcc-media-flows/services/vidispine/vscommon"
	"github.com/bcc-code/bcc-media-flows/workflows/export"
	"github.com/gin-gonic/gin"
	"go.temporal.io/api/history/v1"
	"go.temporal.io/api/workflowservice/v1"
)

type WorkflowListParams struct {
	WorkflowList     []WorkflowDetails
	WorkflowStatuses map[string]string
}

type WorkflowDetails struct {
	VxID       string
	Name       string
	Status     string
	WorkflowID string
	Start      string
}

// Helper function to convert Windows file paths to Linux paths
func convertWindowsPath(windowsPath string) string {
	// Replace backslashes with forward slashes
	path := strings.ReplaceAll(windowsPath, "\\", "/")

	// Remove drive letter if present (e.g., E:)
	if len(path) > 2 && path[1] == ':' {
		path = path[2:]
	}

	return path
}

func (s *TriggerServer) fileCatalystWebhookHandler(ctx *gin.Context) {
	// Extract form parameters from FileCatalyst webhook
	file := ctx.PostForm("f")            // Remote file path
	localFile := ctx.PostForm("lf")      // Local file path
	status := ctx.PostForm("status")     // Status code (1 for success)
	allFiles := ctx.PostForm("allfiles") // All files in the transaction

	// Basic validation
	if file == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"error": "Missing required parameter 'f' (file path)",
		})
		return
	}

	// Log incoming webhook
	fmt.Printf("FileCatalyst webhook: file=%s, localFile=%s, status=%s, allFiles=%s\n",
		file, localFile, status, allFiles)

	// Only proceed if the transfer was successful (status=1)
	if status != "1" {
		ctx.JSON(http.StatusOK, gin.H{
			"message": "Transfer not successful, no signal sent",
			"status":  status,
		})
		return
	}

	// Convert Windows paths to Linux paths for processing
	linuxPath := convertWindowsPath(file)

	// Extract just the filename
	filename := filepath.Base(linuxPath)

	// Send signal to the LIVE-INGEST workflow
	workflowID := "LIVE-INGEST"
	signalName := "file_transferred"

	// Send the signal with just the filename
	err := s.wfClient.SignalWorkflow(ctx, workflowID, "", signalName, filename)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to send signal: %s", err.Error()),
		})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"message":  "Signal sent successfully",
		"filename": filename,
	})
}

func (s *TriggerServer) listGET(ctx *gin.Context) {
	var workflowList []WorkflowDetails

	workflows, err := s.wfClient.ListWorkflow(ctx, &workflowservice.ListWorkflowExecutionsRequest{
		Query: "WorkflowType = 'VXExport'",
	})
	if err != nil {
		renderErrorPage(ctx, http.StatusInternalServerError, err)
		return
	}

	for i := 0; i < len(workflows.Executions); i++ {
		res, err := s.wfClient.WorkflowService().GetWorkflowExecutionHistory(ctx, &workflowservice.GetWorkflowExecutionHistoryRequest{
			Execution: workflows.Executions[i].GetExecution(),
			Namespace: os.Getenv("TEMPORAL_NAMESPACE"),
		})

		if err != nil {
			renderErrorPage(ctx, http.StatusInternalServerError, err)
			return
		}
		attributes, ok := res.History.Events[0].Attributes.(*history.HistoryEvent_WorkflowExecutionStartedEventAttributes)
		if !ok {
			renderErrorPage(ctx, 500, errors.New("unexpected attribute type on first workflow event. Was not HistoryEvent_WorkflowExecutionStartedEventAttributes"))
			break
		}

		data := export.VXExportParams{}
		err = json.Unmarshal(attributes.WorkflowExecutionStartedEventAttributes.Input.Payloads[0].Data, &data)

		if err != nil {
			renderErrorPage(ctx, http.StatusInternalServerError, err)
			return
		}

		meta, err := s.vidispine.GetMetadata(data.VXID)
		if err != nil {
			renderErrorPage(ctx, http.StatusInternalServerError, err)
			return
		}
		name := meta.Get(vscommon.FieldTitle, "")

		loc, _ := time.LoadLocation("Europe/Oslo")
		startTime := workflows.Executions[i].StartTime.AsTime().In(loc).Format("Mon, 02 Jan 2006 15:04:05 MST")

		workflowList = append(workflowList, WorkflowDetails{
			VxID:       data.VXID,
			Name:       name,
			Status:     workflows.Executions[i].GetStatus().String(),
			WorkflowID: workflows.Executions[i].Execution.WorkflowId,
			Start:      startTime,
		})

	}

	workflowStatuses := map[string]string{
		"Running":    "blue",
		"Timed out":  "yellow",
		"Completed":  "green",
		"Failed":     "red",
		"Canceled":   "yellow",
		"Terminated": "red",
	}

	ctx.HTML(http.StatusOK, "list.gohtml", WorkflowListParams{
		WorkflowList:     workflowList,
		WorkflowStatuses: workflowStatuses,
	})
}
