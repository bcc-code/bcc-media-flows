package main

import (
	"net/http"

	"os"

	bccmflows "github.com/bcc-code/bccm-flows"
	"github.com/bcc-code/bccm-flows/common"
	"github.com/bcc-code/bccm-flows/services/vidispine"
	"github.com/bcc-code/bccm-flows/services/vidispine/vsapi"
	"github.com/bcc-code/bccm-flows/services/vidispine/vscommon"
	"github.com/bcc-code/bccm-flows/workflows/export"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
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

	VXID := c.Query("id")
	Languages := c.PostFormArray("languages[]")
	AudioSource := c.PostForm("audioSource")

	queue := getQueue()
	workflowOptions := client.StartWorkflowOptions{
		ID:        uuid.NewString(),
		TaskQueue: queue,
	}

	var res client.WorkflowRun

	s.vsapiClient.SetItemMetadataField(VXID, vscommon.FieldExportAudioSource.Value, AudioSource)

	for i, element := range Languages {
		var err error
		if i == 0 {
			err = s.vsapiClient.SetItemMetadataField(VXID, vscommon.FieldLangsToExport.Value, element)
		} else {
			err = s.vsapiClient.AddToItemMetadataField(VXID, vscommon.FieldLangsToExport.Value, element)

		}

		if err != nil {
			renderErrorPage(c, http.StatusInternalServerError, err)
			return
		}
	}

	res, err := s.wfClient.ExecuteWorkflow(c, workflowOptions, export.VXExport, export.VXExportParams{
		VXID:          VXID,
		WithFiles:     c.PostForm("withFiles") == "on",
		WithChapters:  c.PostForm("withChapters") == "on",
		WatermarkPath: c.PostForm("watermarkPath"),
		AudioSource:   AudioSource,
		Destinations:  c.PostFormArray("destinations[]"),
		Languages:     Languages,
	})

	if err != nil {
		renderErrorPage(c, http.StatusInternalServerError, err)
		return
	}

	// Render success page, with back button
	c.JSON(http.StatusOK, res)

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
	router.POST("/vx-export", server.triggerHandlerPOST)

	router.Run(os.Getenv("PORT"))
}
