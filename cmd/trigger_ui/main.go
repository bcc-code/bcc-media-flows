package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"path/filepath"

	"github.com/bcc-code/bccm-flows/environment"
	"github.com/google/uuid"

	_ "github.com/mattn/go-sqlite3"

	"os"

	bccmflows "github.com/bcc-code/bccm-flows"
	"github.com/bcc-code/bccm-flows/services/vidispine"
	"github.com/bcc-code/bccm-flows/services/vidispine/vsapi"
	"github.com/bcc-code/bccm-flows/services/vidispine/vscommon"
	"github.com/bcc-code/bccm-flows/workflows/export"
	"github.com/gin-gonic/gin"
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
	var filenames []string
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
	vidispine vidispine.Client
	wfClient  client.Client
	languages map[string]bccmflows.Language
	database  *sql.DB
}

func singleValueArrayFromRows(rows *sql.Rows, err error) ([]string, error) {
	if err != nil {
		return nil, err
	}
	defer func() {
		err := rows.Close()
		if err != nil {
			log.Default().Println(err)
		}
	}()

	var array []string
	for rows.Next() {
		var data string
		err = rows.Scan(&data)
		if err != nil {
			return nil, err
		}
		array = append(array, data)
	}
	return array, nil
}

type TriggerGETParams struct {
	Title                   string
	AssetExportDestinations []string
	Filenames               []string
	Languages               map[string]bccmflows.Language
	SelectedLanguages       []string
	SelectedAudioSource     string
	AudioSources            []string
	SubclipNames            []string
}

func (s *TriggerServer) triggerHandlerGET(c *gin.Context) {
	vxID := c.Query("id")
	meta, err := s.vidispine.GetMetadata(vxID)
	if err != nil {
		renderErrorPage(c, http.StatusInternalServerError, err)
		return
	}

	clips := meta.SplitByClips()
	title := clips[vsapi.OriginalClip].Get(vscommon.FieldTitle, "")

	selectedAudioSource := meta.Get(vscommon.FieldExportAudioSource, "")

	selectedLanguages := meta.GetArray(vscommon.FieldLangsToExport)

	filenames, err := getFilenames(overlaysDir)
	if err != nil {
		renderErrorPage(c, http.StatusInternalServerError, err)
		return
	}

	subclipNames, err := vidispine.GetSubclipNames(s.vidispine, vxID)
	if err != nil {
		renderErrorPage(c, http.StatusInternalServerError, err)
		return
	}

	c.HTML(http.StatusOK, "index.gohtml", TriggerGETParams{
		Title:                   title,
		Filenames:               filenames,
		Languages:               s.languages,
		SelectedLanguages:       selectedLanguages,
		SelectedAudioSource:     selectedAudioSource,
		SubclipNames:            subclipNames,
		AudioSources:            vidispine.ExportAudioSources.Values(),
		AssetExportDestinations: export.AssetExportDestinations.Values(),
	})
}

func (s *TriggerServer) triggerHandlerPOST(c *gin.Context) {
	vxID := c.Query("id")
	languages := c.PostFormArray("languages[]")
	audioSource := c.PostForm("audioSource")

	queue := getQueue()
	workflowOptions := client.StartWorkflowOptions{
		TaskQueue: queue,
	}

	if os.Getenv("DEBUG") == "" {
		workflowOptions.SearchAttributes = map[string]any{
			"CustomStringField": vxID,
		}
	}

	err := s.vidispine.SetItemMetadataField(vxID, vscommon.FieldExportAudioSource.Value, audioSource)
	if err != nil {
		renderErrorPage(c, http.StatusInternalServerError, err)
		return
	}

	for i, element := range languages {
		if i == 0 {
			err = s.vidispine.SetItemMetadataField(vxID, vscommon.FieldLangsToExport.Value, element)
		} else {
			err = s.vidispine.AddToItemMetadataField(vxID, vscommon.FieldLangsToExport.Value, element)

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

	params := export.VXExportParams{
		VXID:          vxID,
		WithFiles:     c.PostForm("withFiles") == "on",
		WithChapters:  c.PostForm("withChapters") == "on",
		WatermarkPath: watermarkPath,
		AudioSource:   audioSource,
		Destinations:  c.PostFormArray("destinations[]"),
		Languages:     languages,
	}

	var wfID string

	subclips := c.PostFormArray("subclips[]")
	if len(subclips) > 0 {
		for _, subclip := range subclips {
			params.Subclip = subclip
			workflowOptions.ID = uuid.NewString()
			_, err = s.wfClient.ExecuteWorkflow(c, workflowOptions, export.VXExport, params)
			if err != nil {
				renderErrorPage(c, http.StatusInternalServerError, err)
				return
			}
		}
	} else {
		workflowOptions.ID = uuid.NewString()
		res, err := s.wfClient.ExecuteWorkflow(c, workflowOptions, export.VXExport, params)

		if err != nil {
			renderErrorPage(c, http.StatusInternalServerError, err)
			return
		}

		wfID = res.GetID()
	}

	meta, err := s.vidispine.GetMetadata(vxID)
	if err != nil {
		renderErrorPage(c, http.StatusInternalServerError, err)
		return
	}

	// Render success page, with back button

	c.HTML(http.StatusOK, "success.gohtml", gin.H{
		"WorkflowID": wfID,
		"Title":      meta.Get(vscommon.FieldTitle, ""),
	})
}

func main() {
	router := gin.Default()

	vsapiClient := vsapi.NewClient(os.Getenv("VIDISPINE_BASE_URL"), os.Getenv("VIDISPINE_USERNAME"), os.Getenv("VIDISPINE_PASSWORD"))
	wfClient, err := getTemporalClient()
	if err != nil {
		panic(err.Error())
	}
	lang := bccmflows.LanguagesByISO

	router.LoadHTMLGlob("./templates/*")

	sqlitePath, ok := os.LookupEnv("TRIGGER_DB")
	if !ok {
		panic("No TRIGGER_DB environment variable.")
	}

	db, err := sql.Open("sqlite3", sqlitePath)
	if err != nil {
		panic(err.Error())
	}

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS tags (
		name TEXT NOT NULL UNIQUE 
	);
	CREATE TABLE IF NOT EXISTS persons (
		name TEXT NOT NULL UNIQUE
	);
	CREATE TABLE IF NOT EXISTS program_ids (
		name TEXT NOT NULL UNIQUE 
	);`)
	if err != nil {
		panic(err.Error())
	}

	server := &TriggerServer{
		vsapiClient,
		wfClient,
		lang,
		db,
	}

	router.Group("/vx-export").
		GET("/", server.triggerHandlerGET).
		GET("/list", server.listGET).
		POST("/", server.triggerHandlerPOST)

	router.Group("/upload-master").
		GET("/", server.uploadMasterGET).
		POST("/", server.uploadMasterPOST).
		GET("/admin", server.uploadMasterAdminGET).
		POST("/admin", server.uploadMasterAdminPOST)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8083"
	}

	fmt.Printf("Started on port %s", port)
	err = router.Run(fmt.Sprintf(":%s", port))
	if err != nil {
		panic(err)
	}
}
