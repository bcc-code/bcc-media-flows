package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/bcc-code/bccm-flows/environment"
	"github.com/bcc-code/bccm-flows/paths"

	_ "github.com/mattn/go-sqlite3"

	"os"

	bccmflows "github.com/bcc-code/bccm-flows"
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
		queue = environment.QueueWorker
	}
	return queue
}

var overlaysDir = os.Getenv("OVERLAYS_DIR")
var masterTriggerDir = os.Getenv("MASTER_TRIGGER_DIR")

func getFilenames(dir string) ([]string, error) {
	files, err := os.ReadDir(dir)
	filenames := []string{}
	if err != nil {
		return filenames, err
	}

	for _, file := range files {
		filenames = append(filenames, file.Name())
	}
	return filenames, nil
}

func getOverlayFilePath(file string) string {
	return filepath.Join(overlaysDir, file)
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

type TriggerGETParams struct {
	Title                   string
	AssetExportDestinations []string
	Filenames               []string
	Languages               map[string]bccmflows.Language
	SelectedLanguages       []string
	SelectedAudioSource     string
	AudioSources            []string
}

func (s *TriggerServer) triggerHandlerGET(c *gin.Context) {
	meta, err := s.vsapiClient.GetMetadata(c.Query("id"))
	if err != nil {
		renderErrorPage(c, http.StatusInternalServerError, err)
		return
	}

	title := meta.Get(vscommon.FieldTitle, "")

	selectedAudioSource := meta.Get(vscommon.FieldExportAudioSource, "")

	selectedLanguages := meta.GetArray(vscommon.FieldLangsToExport)

	filenames, err := getFilenames(overlaysDir)
	if err != nil {
		renderErrorPage(c, http.StatusInternalServerError, err)
		return
	}

	c.HTML(http.StatusOK, "index.gohtml", TriggerGETParams{
		Title:                   title,
		AssetExportDestinations: s.assetExportDestinations,
		Filenames:               filenames,
		Languages:               s.languages,
		SelectedLanguages:       selectedLanguages,
		SelectedAudioSource:     selectedAudioSource,
		AudioSources:            s.ExportAudioSources,
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

	if os.Getenv("DEBUG") == "" {
		workflowOptions.SearchAttributes = map[string]any{
			"CustomStringField": vxID,
		}
	}

	err := s.vsapiClient.SetItemMetadataField(vxID, vscommon.FieldExportAudioSource.Value, audioSource)
	if err != nil {
		renderErrorPage(c, http.StatusInternalServerError, err)
		return
	}

	for i, element := range languages {
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

	var watermarkPath string
	watermarkFile := c.PostForm("watermarkFile")
	if watermarkFile != "" {
		watermarkPath = getOverlayFilePath(watermarkFile)
	}

	res, err := s.wfClient.ExecuteWorkflow(c, workflowOptions, export.VXExport, export.VXExportParams{
		VXID:          vxID,
		WithFiles:     c.PostForm("withFiles") == "on",
		WithChapters:  c.PostForm("withChapters") == "on",
		WatermarkPath: watermarkPath,
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

func (s *TriggerServer) listGET(c *gin.Context) {
	var workflowList []WorkflowDetails

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
		startTime := workflows.Executions[i].StartTime.In(loc).Format("Mon, 02 Jan 2006 15:04:05 MST")

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

	c.HTML(http.StatusOK, "list.gohtml", WorkflowListParams{
		WorkflowList:     workflowList,
		WorkflowStatuses: workflowStatuses,
	})
}

func (s *TriggerServer) uploadMasterGET(c *gin.Context) {
	filenames, err := getFilenames(masterTriggerDir)
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

	s.addDataToTable(c, c.PostFormArray("tags[]"), "tags")
	s.addDataToTable(c, c.PostFormArray("persons[]"), "persons")

	path := masterTriggerDir + "/" + c.PostForm("path")

	_, err := s.wfClient.ExecuteWorkflow(c, workflowOptions, ingestworkflows.Masters, ingestworkflows.MasterParams{
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
		SourceFile: paths.MustParse(path),
	})

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

	DeleteRow := c.PostFormArray("deleteArrayData[]")

	if DeleteRow != nil {
		_, err := s.database.Exec("DELETE FROM programID WHERE name='" + DeleteRow[0] + "'")
		if err != nil {
			renderErrorPage(c, http.StatusInternalServerError, err)
			return
		}
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

	router.LoadHTMLGlob("./templates/*")

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
