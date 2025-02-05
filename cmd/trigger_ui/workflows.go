package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
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
