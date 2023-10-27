package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"os"

	bccmflows "github.com/bcc-code/bccm-flows"
	"github.com/bcc-code/bccm-flows/common"
	"github.com/bcc-code/bccm-flows/services/vidispine"
	"github.com/bcc-code/bccm-flows/services/vidispine/vsapi"
	"github.com/bcc-code/bccm-flows/services/vidispine/vscommon"
	"github.com/bcc-code/bccm-flows/workflows/export"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.temporal.io/api/history/v1"
	"go.temporal.io/api/workflowservice/v1"
	"go.temporal.io/sdk/client"
)

func getTemporalClient() (client.Client, error) {
	return client.Dial(client.Options{
		HostPort:  os.Getenv("TEMPORAL_HOST_PORT"),
		Namespace: os.Getenv("TEMPORAL_NAMESPACE"),
	})
}

func getQueue() string {
	queue := os.Getenv("QUEUE")
	if queue == "" {
		queue = common.QueueWorker
	}
	return queue
}

func getOverlayFilenames() ([]string, error) {
	files, err := os.ReadDir(os.Getenv("OVERLAYS_DIR"))
	filenames := []string{}
	if err != nil {
		return filenames, err
	}

	for _, file := range files {
		filenames = append(filenames, file.Name())
	}
	return filenames, nil
}

func renderErrorPage(c *gin.Context, httpStatus int, err error) {
	c.HTML(httpStatus, "error.html", gin.H{
		"errorMessage": err.Error(),
	})
}

type TriggerServer struct {
	vsapiClient             *vsapi.Client
	assetExportDestinations []string
	wfClient                client.Client
	languages               map[string]bccmflows.Language
	ExportAudioSources      []string
}

func (s *TriggerServer) triggerHandlerGET(c *gin.Context) {
	meta, err := s.vsapiClient.GetMetadata(c.Query("id"))
	if err != nil {
		renderErrorPage(c, http.StatusInternalServerError, err)
		return
	}

	title := meta.Get(vscommon.FieldTitle, "")

	selectedAudiosource := meta.Get(vscommon.FieldExportAudioSource, "")

	selectedLanguages := meta.GetArray(vscommon.FieldLangsToExport)

	filenames, err := getOverlayFilenames()
	if err != nil {
		renderErrorPage(c, http.StatusInternalServerError, err)
		return
	}

	c.HTML(http.StatusOK, "index.html", gin.H{
		"Title":                   title,
		"AssetExportDestinations": s.assetExportDestinations,
		"Filenames":               filenames,
		"Languages":               s.languages,
		"SelectedLanguages":       selectedLanguages,
		"SelectedAudioSource":     selectedAudiosource,
		"AudioSources":            s.ExportAudioSources,
	})
}

func (s *TriggerServer) triggerHandlerPOST(c *gin.Context) {

	vxID := c.Query("id")
	languages := c.PostFormArray("languages[]")
	audioSource := c.PostForm("audioSource")

	queue := getQueue()
	workflowOptions := client.StartWorkflowOptions{
		ID:        uuid.NewString(),
		TaskQueue: queue,
	}

	var res client.WorkflowRun

	s.vsapiClient.SetItemMetadataField(vxID, vscommon.FieldExportAudioSource.Value, audioSource)

	for i, element := range languages {
		var err error
		if i == 0 {
			err = s.vsapiClient.SetItemMetadataField(vxID, vscommon.FieldLangsToExport.Value, element)
		} else {
			err = s.vsapiClient.AddToItemMetadataField(vxID, vscommon.FieldLangsToExport.Value, element)

		}

		if err != nil {
			renderErrorPage(c, http.StatusInternalServerError, err)
			return
		}
	}

	res, err := s.wfClient.ExecuteWorkflow(c, workflowOptions, export.VXExport, export.VXExportParams{
		VXID:          vxID,
		WithFiles:     c.PostForm("withFiles") == "on",
		WithChapters:  c.PostForm("withChapters") == "on",
		WatermarkPath: c.PostForm("watermarkPath"),
		AudioSource:   audioSource,
		Destinations:  c.PostFormArray("destinations[]"),
		Languages:     languages,
	})

	if err != nil {
		renderErrorPage(c, http.StatusInternalServerError, err)
		return
	}

	meta, err := s.vsapiClient.GetMetadata(vxID)
	if err != nil {
		renderErrorPage(c, http.StatusInternalServerError, err)
		return
	}

	// Render success page, with back button

	c.HTML(http.StatusOK, "success.html", gin.H{
		"WorkflowID": res.GetID(),
		"Title":      meta.Get(vscommon.FieldTitle, ""),
	})

}

type workflowStruct struct {
	VxID       string
	Name       string
	Status     string
	WorkflowID string
	Start      string
}

func (s *TriggerServer) listGET(c *gin.Context) {

	workflowList := []workflowStruct{}

	workflows, err := s.wfClient.ListWorkflow(c, &workflowservice.ListWorkflowExecutionsRequest{
		Query: "WorkflowType = 'VXExport'",
	})
	if err != nil {
		renderErrorPage(c, http.StatusInternalServerError, err)
		return
	}

	for i := 0; i < len(workflows.Executions); i++ {
		res, err := s.wfClient.WorkflowService().GetWorkflowExecutionHistory(c, &workflowservice.GetWorkflowExecutionHistoryRequest{
			Execution: workflows.Executions[i].GetExecution(),
			Namespace: os.Getenv("TEMPORAL_NAMESPACE"),
		})
		print(res)
		if err != nil {
			renderErrorPage(c, http.StatusInternalServerError, err)
			return
		}
		attributes, ok := res.History.Events[0].Attributes.(*history.HistoryEvent_WorkflowExecutionStartedEventAttributes)
		if !ok {
			renderErrorPage(c, 500, errors.New("unexpected attribute type on first workflow event. Was not HistoryEvent_WorkflowExecutionStartedEventAttributes"))
			break
		}
		data := export.VXExportParams{}
		err = json.Unmarshal(attributes.WorkflowExecutionStartedEventAttributes.Input.Payloads[0].Data, &data)

		if err != nil {
			renderErrorPage(c, http.StatusInternalServerError, err)
			return
		}

		meta, err := s.vsapiClient.GetMetadata(data.VXID)
		if err != nil {
			renderErrorPage(c, http.StatusInternalServerError, err)
			return
		}
		name := meta.Get(vscommon.FieldTitle, "")

		loc, _ := time.LoadLocation("Europe/Oslo")
		startime := workflows.Executions[i].StartTime.In(loc).Format("Mon, 02 Jan 2006 15:04:05 MST")
		workflowList = append(workflowList, workflowStruct{
			VxID:       data.VXID,
			Name:       name,
			Status:     workflows.Executions[i].GetStatus().String(),
			WorkflowID: workflows.Executions[i].Execution.WorkflowId,
			Start:      startime,
		})

	}

	c.HTML(http.StatusOK, "list.html", gin.H{
		"WorkflowList": workflowList,
	})

}

func main() {
	router := gin.Default()

	assetExportDestinations := export.AssetExportDestinations.Values()
	ExportAudioSources := vidispine.ExportAudioSources.Values()
	vsapiClient := vsapi.NewClient(os.Getenv("VIDISPINE_BASE_URL"), os.Getenv("VIDISPINE_USERNAME"), os.Getenv("VIDISPINE_PASSWORD"))
	wfClient, err := getTemporalClient()
	if err != nil {
		panic(err.Error())
	}
	lang := bccmflows.LanguagesByISO

	router.LoadHTMLGlob("*.html")
	router.Static("/css", "../trigger_ui/css")

	server := &TriggerServer{
		vsapiClient,
		assetExportDestinations,
		wfClient,
		lang,
		ExportAudioSources,
	}

	router.GET("/vx-export", server.triggerHandlerGET)
	router.GET("/vx-export/list", server.listGET)
	router.POST("/vx-export", server.triggerHandlerPOST)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8083"
	}
	router.Run(fmt.Sprintf(":%s", port))
}
