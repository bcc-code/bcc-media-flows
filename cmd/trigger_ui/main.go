package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"sort"

	"github.com/bcc-code/bcc-media-flows/environment"
	"github.com/bcc-code/bcc-media-platform/backend/asset"
	"github.com/bcc-code/bcc-media-platform/backend/utils"
	"github.com/google/uuid"
	"github.com/samber/lo"

	_ "github.com/glebarez/go-sqlite"

	"os"

	bccmflows "github.com/bcc-code/bcc-media-flows"
	"github.com/bcc-code/bcc-media-flows/services/vidispine"
	"github.com/bcc-code/bcc-media-flows/services/vidispine/vsapi"
	"github.com/bcc-code/bcc-media-flows/services/vidispine/vscommon"
	"github.com/bcc-code/bcc-media-flows/workflows/export"
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
	Resolutions             []vsapi.Resolution
	Ratio                   string
}

func ratio(w, h int) string {
	a := w
	b := h

	for b != 0 {
		t := b
		b = a % b
		a = t
	}

	return fmt.Sprintf("%d:%d", w/a, h/a)
}

func (s *TriggerServer) triggerHandlerGET(c *gin.Context) {
	vxID := c.Query("id")
	meta, err := s.vidispine.GetMetadata(vxID)
	if err != nil {
		renderErrorPage(c, http.StatusInternalServerError, err)
		return
	}

	resolutions, err := s.vidispine.GetResolutions(vxID)
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

	exportData, err := vidispine.GetDataForExport(s.vidispine, vxID, nil, nil, "")
	if err != nil {
		renderErrorPage(c, http.StatusInternalServerError, err)
		return
	}
	chapters, err := vidispine.GetChapterData(s.vidispine, exportData)
	if err != nil {
		renderErrorPage(c, http.StatusInternalServerError, err)
		return
	}
	sort.Slice(chapters, func(i, j int) bool {
		return chapters[i].Timestamp < chapters[j].Timestamp
	})
	subclipNames := lo.Map(chapters, func(c asset.Chapter, _ int) string {
		return c.Title
	})

	var ratioString string

	if len(resolutions) > 0 {
		ratioString = ratio(resolutions[0].Width, resolutions[0].Height)
	}

	c.HTML(http.StatusOK, "vx-export.gohtml", TriggerGETParams{
		Title:                   title,
		Filenames:               filenames,
		Languages:               s.languages,
		SelectedLanguages:       selectedLanguages,
		SelectedAudioSource:     selectedAudioSource,
		SubclipNames:            subclipNames,
		AudioSources:            vidispine.ExportAudioSources.Values(),
		AssetExportDestinations: export.AssetExportDestinations.Values(),
		Resolutions:             resolutions,
		Ratio:                   ratioString,
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

	go func() {
		err := s.vidispine.SetItemMetadataField(vxID, "", vscommon.FieldExportAudioSource.Value, audioSource)
		if err != nil {
			log.Default().Println(err)
		}

		for i, element := range languages {
			if i == 0 {
				err = s.vidispine.SetItemMetadataField(vxID, "", vscommon.FieldLangsToExport.Value, element)
			} else {
				err = s.vidispine.AddToItemMetadataField(vxID, "", vscommon.FieldLangsToExport.Value, element)

			}

			if err != nil {
				log.Default().Println(err)
			}
		}
	}()

	var watermarkPath string
	watermarkFile := c.PostForm("watermarkFile")
	if watermarkFile != "" {
		watermarkPath = getOverlayFilePath(watermarkFile)
	}

	resolutionIndexes := lo.Map(c.PostFormArray("resolutions[]"), func(i string, _ int) int {
		return utils.AsInt(i)
	})
	fileIndexes := lo.Map(c.PostFormArray("files[]"), func(i string, _ int) int {
		return utils.AsInt(i)
	})

	vsresolutions, err := s.vidispine.GetResolutions(vxID)
	if err != nil {
		renderErrorPage(c, http.StatusInternalServerError, err)
		return
	}

	var selectedResolutions []export.Resolution
	for _, i := range resolutionIndexes {
		r := vsresolutions[i]
		selectedResolutions = append(selectedResolutions, export.Resolution{
			Width:  r.Width,
			Height: r.Height,
			File:   lo.Contains(fileIndexes, i),
		})
	}

	params := export.VXExportParams{
		VXID:          vxID,
		WithChapters:  c.PostForm("withChapters") == "on",
		IgnoreSilence: c.PostForm("ignoreSilence") == "on",
		WatermarkPath: watermarkPath,
		AudioSource:   audioSource,
		Destinations:  c.PostFormArray("destinations[]"),
		Languages:     languages,
		Resolutions:   selectedResolutions,
	}

	var wfID string

	subclips := c.PostFormArray("subclips[]")
	if len(subclips) > 0 {
		for _, subclip := range subclips {
			params.Subclip = subclip
			workflowOptions.ID = uuid.NewString()
			_, err := s.wfClient.ExecuteWorkflow(c, workflowOptions, export.VXExport, params)
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

	db, err := sql.Open("sqlite", sqlitePath)
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

	router.GET("/list", server.listGET)

	router.Group("/vx-export").
		GET("/", server.triggerHandlerGET).
		POST("/", server.triggerHandlerPOST)

	router.Group("/vb-export").
		GET("/", server.VBTriggerHandlerGET).
		POST("/", server.VBTriggerHandlerPOST)

	router.Group("/upload-master").
		GET("/", server.uploadMasterGET).
		POST("/", server.uploadMasterPOST).
		GET("/admin", server.uploadMasterAdminGET).
		POST("/admin", server.uploadMasterAdminPOST)

	router.Group("/ingest-fix").
		GET("/", server.ingestFixGET).
		POST("/mu1mu2extract", server.mu1mu2ExtractPOST).
		GET("/sync", server.ingestSyncFixGET).
		POST("/sync", server.ingestSyncFixPOST)

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
