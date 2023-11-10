package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"os"

	bccmflows "github.com/bcc-code/bccm-flows"
	"github.com/bcc-code/bccm-flows/common"
	"github.com/bcc-code/bccm-flows/services/ingest"
	"github.com/bcc-code/bccm-flows/services/vidispine"
	"github.com/bcc-code/bccm-flows/services/vidispine/vsapi"
	"github.com/bcc-code/bccm-flows/services/vidispine/vscommon"
	"github.com/bcc-code/bccm-flows/workflows/export"
	ingestworkflows "github.com/bcc-code/bccm-flows/workflows/ingest"
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

func getOverlayFilenames(env string) ([]string, error) {
	files, err := os.ReadDir(os.Getenv(env))
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
	c.HTML(httpStatus, "error.gohtml", gin.H{
		"errorMessage": err.Error(),
	})
}

type TriggerServer struct {
	vsapiClient             *vsapi.Client
	assetExportDestinations []string
	wfClient                client.Client
	languages               map[string]bccmflows.Language
	ExportAudioSources      []string
	database                *sql.DB
}

func (s *TriggerServer) getArrayfromTable(c *gin.Context, table string) []string {
	rows, err := s.database.Query("SELECT name FROM " + table)
	if err != nil {
		renderErrorPage(c, http.StatusInternalServerError, err)
		return nil
	}

	array := []string{}
	for rows.Next() {
		var data string
		err = rows.Scan(&data)
		if err != nil {
			renderErrorPage(c, http.StatusInternalServerError, err)
			return nil
		}
		array = append(array, data)
	}
	return array
}

func (s *TriggerServer) addDataToTable(c *gin.Context, array []string, table string) {

	tableArray := s.getArrayfromTable(c, table)

	for i := 0; i < len(array); i++ {

		isSame := false

		for j := 0; j < len(tableArray); j++ {
			if tableArray[j] == array[i] {
				isSame = true
			}
		}

		if !isSame {
			_, err := s.database.Exec("INSERT INTO " + table + " (name) VALUES ('" + array[i] + "')")
			if err != nil {
				renderErrorPage(c, http.StatusInternalServerError, err)
				return
			}
		}

	}
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

	filenames, err := getOverlayFilenames("OVERLAYS_DIR")
	if err != nil {
		renderErrorPage(c, http.StatusInternalServerError, err)
		return
	}

	c.HTML(http.StatusOK, "index.gohtml", gin.H{
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

	c.HTML(http.StatusOK, "success.gohtml", gin.H{
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

	workflowStatuses := map[string]string{
		"Running":    "blue",
		"Timed out":  "yellow",
		"Completed":  "green",
		"Failed":     "red",
		"Canceled":   "yellow",
		"Terminated": "red",
	}

	c.HTML(http.StatusOK, "list.html", gin.H{
		"WorkflowList":     workflowList,
		"WorkflowStatuses": workflowStatuses,
	})

}

func (s *TriggerServer) uploadMasterGET(c *gin.Context) {
	filenames, err := getOverlayFilenames("MASTER_TRIGGER_DIR")
	if err != nil {
		renderErrorPage(c, http.StatusInternalServerError, err)
		return
	}

	tags := s.getArrayfromTable(c, "tags")

	persons := s.getArrayfromTable(c, "persons")

	programID := s.getArrayfromTable(c, "programID")

	c.HTML(http.StatusOK, "upload-master.gohtml", gin.H{
		"fileDirectory":   filenames,
		"TagsDatalist":    tags,
		"PersonsDatalist": persons,
		"programIDs":      programID,
	})
}

func (s *TriggerServer) uploadMasterPOST(c *gin.Context) {

	queue := getQueue()
	workflowOptions := client.StartWorkflowOptions{
		ID:        uuid.NewString(),
		TaskQueue: queue,
	}

	var res client.WorkflowRun

	s.addDataToTable(c, c.PostFormArray("tags[]"), "tags")
	s.addDataToTable(c, c.PostFormArray("persons[]"), "persons")

	res, err := s.wfClient.ExecuteWorkflow(c, workflowOptions, ingestworkflows.VBMaster, ingestworkflows.VBMasterParams{
		Metadata: &ingest.Metadata{
			JobProperty: ingest.JobProperty{
				ProgramID:        c.PostForm("program_id"),
				Tags:             strings.Join(c.PostFormArray("tags[]"), ", "),
				PersonsAppearing: strings.Join(c.PostFormArray("persons[]"), ", "),
				SenderEmail:      c.PostForm("sender_email"),
				Language:         c.PostForm("language"),
				ReceivedFilename: c.PostForm("filename"),
			},
		},
		/* SourceFile: c.PostForm("path"), */
	})

	res.GetID()

	if err != nil {
		renderErrorPage(c, http.StatusInternalServerError, err)
		return
	}

}

func (s *TriggerServer) uploadMasterAdminGET(c *gin.Context) {

	programID := s.getArrayfromTable(c, "programID")

	c.HTML(http.StatusOK, "upload-master-admin.gohtml", gin.H{
		"programIDArray": programID,
	})
}

func (s *TriggerServer) uploadMasterAdminPOST(c *gin.Context) {

	programID := []string{(c.PostForm("Code") + " - " + c.PostForm("Name"))}

	if programID[0] != " - " {
		s.addDataToTable(c, programID, "programID")
	}

	_, err := s.database.Exec("DELETE FROM programID WHERE name='" + c.PostForm("") + "'")
	if err != nil {
		renderErrorPage(c, http.StatusInternalServerError, err)
		return
	}

	s.uploadMasterAdminGET(c)
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

	router.LoadHTMLGlob("*.gohtml")

	sqlite_path, exists := os.LookupEnv("TRIGGER_DB")
	if !exists {
		panic("No TRIGGER_DB environment variable.")
	}

	db, err := sql.Open("sqlite3", sqlite_path)
	if err != nil {
		panic(err.Error())
	}

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS tags (
		name TEXT NOT NULL
	);
	CREATE TABLE IF NOT EXISTS persons (
		name TEXT NOT NULL
	);
	CREATE TABLE IF NOT EXISTS programID (
		name TEXT NOT NULL
	);
	`)
	if err != nil {
		panic(err.Error())
	}

	server := &TriggerServer{
		vsapiClient,
		assetExportDestinations,
		wfClient,
		lang,
		ExportAudioSources,
		db,
	}

	router.GET("/vx-export", server.triggerHandlerGET)
	router.GET("/vx-export/list", server.listGET)
	router.GET("/upload-master", server.uploadMasterGET)
	router.GET("/upload-master/admin", server.uploadMasterAdminGET)
	router.POST("/upload-master/admin", server.uploadMasterAdminPOST)
	router.POST("/vx-export", server.triggerHandlerPOST)
	router.POST("/upload-master", server.uploadMasterPOST)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8083"
	}

	fmt.Printf("Started on port %s", port)
	router.Run(fmt.Sprintf(":%s", port))
}
