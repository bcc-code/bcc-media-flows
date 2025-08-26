package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	miscworkflows "github.com/bcc-code/bcc-media-flows/workflows/misc"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	common "go.temporal.io/api/common/v1"
	workflowservice "go.temporal.io/api/workflowservice/v1"
	"go.temporal.io/sdk/client"
)

type WorkflowListParams struct {
	WorkflowList     []WorkflowDetails
	WorkflowStatuses map[string]string
}

// Parse JSON body
type massiveObject struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Sender     string `json:"sender"`
	TotalFiles int    `json:"total_files"`
	State      string `json:"state"`
	Type       string `json:"type"`
}
type massivePayload struct {
	EventID   string        `json:"event_id"`
	EventTime string        `json:"event_time"`
	EventType string        `json:"event_type"`
	Object    massiveObject `json:"object"`
}

// Massive.app webhook handler
// Expects:
//   - Header: api-key (must match MASSIVE_WEBHOOK_API_KEY env var if set)
//   - JSON body similar to:
//     {
//     "event_type": "package.finalized",
//     "object": {"id": "...", "name": "...", "sender": "...", "total_files": 1}
//     }
func (s *TriggerServer) massiveWebhookHandler(ctx *gin.Context) {
	// Validate API key if configured
	expectedKey := os.Getenv("MASSIVE_WEBHOOK_API_KEY")
	if expectedKey != "" {
		if ctx.GetHeader("api-key") != expectedKey {
			ctx.JSON(http.StatusUnauthorized, gin.H{"error": "invalid api-key"})
			return
		}
	}
	var payload massivePayload
	if err := ctx.ShouldBindJSON(&payload); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON payload", "details": err.Error()})
		return
	}

	// Log incoming webhook
	fmt.Printf("Massive.app webhook: type=%s, id=%s, name=%s, sender=%s, total_files=%d, state=%s\n",
		payload.EventType, payload.Object.ID, payload.Object.Name, payload.Object.Sender, payload.Object.TotalFiles, payload.Object.State)

	// Only act on finalized packages
	if payload.EventType != "package.finalized" {
		ctx.JSON(http.StatusOK, gin.H{"message": "event ignored", "event_type": payload.EventType})
		return
	}

	// Start MASVImport workflow
	queue := getQueue()
	options := client.StartWorkflowOptions{TaskQueue: queue}
	options.ID = uuid.NewString() + "-" + payload.Object.ID

	params := miscworkflows.MASVImportParams{
		ID:         payload.Object.ID,
		Name:       payload.Object.Name,
		Sender:     payload.Object.Sender,
		TotalFiles: payload.Object.TotalFiles,
		EventID:    payload.EventID,
		EventTime:  payload.EventTime,
	}

	_, err := s.wfClient.ExecuteWorkflow(ctx, options, miscworkflows.MASVImport, params)
	if err != nil {
		log.Default().Println(err)
		ctx.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	ctx.Status(http.StatusAccepted)
}

type WorkflowDetails struct {
	VxID       string
	Name       string
	Status     string
	WorkflowID string
	Start      string
}

func (s *TriggerServer) fileCatalystWebhookHandler(ctx *gin.Context) {
	file := ctx.PostForm("f")            // Remote file path
	localFile := ctx.PostForm("lf")      // Local file path
	status := ctx.PostForm("status")     // Status code (1 for success)
	allFiles := ctx.PostForm("allfiles") // All files in the transaction

	// Basic validation
	if file == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Missing file parameter"})
		return
	}

	// Log incoming webhook
	fmt.Printf("FileCatalyst webhook: file=%s, localFile=%s, status=%s, allFiles=%s\n",
		file, localFile, status, allFiles)

	// Only proceed if the transfer was successful (status=1)
	if status != "1" {
		ctx.JSON(http.StatusOK, gin.H{
			"message":  "Transfer not successful, signal not sent",
			"filename": file,
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

func (s *TriggerServer) listGET(ctx *gin.Context) {
	var workflowList []WorkflowDetails

	workflows, err := s.wfClient.ListWorkflow(ctx, &workflowservice.ListWorkflowExecutionsRequest{
		Query: "",
	})
	if err != nil {
		renderErrorPage(ctx, http.StatusInternalServerError, err)
		return
	}

	for _, exec := range workflows.Executions {
		// Only include workflows with no parent (parentless)
		if exec.ParentExecution != nil {
			continue
		}
		workflowList = append(workflowList, WorkflowDetails{
			VxID:       "",
			Name:       exec.Type.GetName(),
			Status:     exec.GetStatus().String(),
			WorkflowID: exec.Execution.GetWorkflowId(),
			Start:      exec.GetStartTime().AsTime().Format("2006-01-02 15:04:05"),
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

func (s *TriggerServer) workflowDetailsGET(ctx *gin.Context) {
	workflowID := ctx.Param("id")
	namespace := os.Getenv("TEMPORAL_NAMESPACE")
	resp, err := s.wfClient.WorkflowService().GetWorkflowExecutionHistory(ctx, &workflowservice.GetWorkflowExecutionHistoryRequest{
		Execution: &common.WorkflowExecution{
			WorkflowId: workflowID,
		},
		Namespace: namespace,
	})
	if err != nil {
		ctx.HTML(http.StatusOK, "workflow-details.gohtml", gin.H{"Error": err.Error()})
		return
	}

	// Extract status, start time, and type from history/events if possible
	var status, start, wfType string
	if len(resp.History.Events) > 0 {
		for _, event := range resp.History.Events {
			if event.GetEventType().String() == "EVENT_TYPE_WORKFLOW_EXECUTION_STARTED" {
				start = event.GetEventTime().AsTime().Format("2006-01-02 15:04:05")
				if attr := event.GetWorkflowExecutionStartedEventAttributes(); attr != nil {
					wfType = attr.WorkflowType.GetName()
				}
			}
			if event.GetEventType().String() == "EVENT_TYPE_WORKFLOW_EXECUTION_COMPLETED" {
				status = "Completed"
			}
			if event.GetEventType().String() == "EVENT_TYPE_WORKFLOW_EXECUTION_FAILED" {
				status = "Failed"
			}
			if event.GetEventType().String() == "EVENT_TYPE_WORKFLOW_EXECUTION_CANCELED" {
				status = "Canceled"
			}
			if event.GetEventType().String() == "EVENT_TYPE_WORKFLOW_EXECUTION_TERMINATED" {
				status = "Terminated"
			}
			if event.GetEventType().String() == "EVENT_TYPE_WORKFLOW_EXECUTION_TIMED_OUT" {
				status = "Timed out"
			}
		}
	}
	if status == "" {
		status = "Running"
	}

	// Query for all workflows whose ParentExecution.WorkflowId == workflowID
	childrenResp, err := s.wfClient.ListWorkflow(ctx, &workflowservice.ListWorkflowExecutionsRequest{
		Query: fmt.Sprintf("ParentWorkflowId='%s'", workflowID),
	})
	var children []WorkflowDetails
	if err == nil {
		for _, child := range childrenResp.Executions {
			children = append(children, WorkflowDetails{
				VxID:       "",
				Name:       child.Type.GetName(),
				Status:     child.GetStatus().String(),
				WorkflowID: child.Execution.GetWorkflowId(),
				Start:      child.GetStartTime().AsTime().Format("2006-01-02 15:04:05"),
			})
		}
	}

	historyJson, _ := json.MarshalIndent(resp.History, "", "  ")
	ctx.HTML(http.StatusOK, "workflow-details.gohtml", gin.H{
		"WorkflowID": workflowID,
		"Status":     status,
		"Start":      start,
		"Type":       wfType,
		"History":    string(historyJson),
		"Children":   children,
	})
}
