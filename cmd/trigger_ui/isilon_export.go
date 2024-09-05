package main

import (
	"net/http"
	"os"

	"github.com/bcc-code/bcc-media-flows/services/vidispine"
	"github.com/bcc-code/bcc-media-flows/services/vidispine/vsapi"
	"github.com/bcc-code/bcc-media-flows/services/vidispine/vscommon"
	"github.com/bcc-code/bcc-media-flows/utils"
	"github.com/bcc-code/bcc-media-flows/workflows/export"
	bccmUtils "github.com/bcc-code/bcc-media-platform/backend/utils"
	"github.com/davecgh/go-spew/spew"
	"github.com/gin-gonic/gin"
	"github.com/teris-io/shortid"
	"go.temporal.io/sdk/client"
)

func (s *TriggerServer) isilonExportGET(ctx *gin.Context) {
	vxID := ctx.Query("id")
	meta, err := s.vidispine.GetMetadata(vxID)
	if err != nil {
		renderErrorPage(ctx, http.StatusInternalServerError, err)
		return
	}

	resolutions, err := s.vidispine.GetResolutions(vxID)
	if err != nil {
		renderErrorPage(ctx, http.StatusInternalServerError, err)
		return
	}

	clips := meta.SplitByClips()
	title := clips[vsapi.OriginalClip].Get(vscommon.FieldTitle, "")

	selectedAudioSource := meta.Get(vscommon.FieldExportAudioSource, "")

	filenames, err := getFilenames(overlaysDir)
	if err != nil {
		renderErrorPage(ctx, http.StatusInternalServerError, err)
		return
	}

	var ratioString string

	if len(resolutions) > 0 {
		ratioString = ratio(resolutions[0].Width, resolutions[0].Height)
	}

	ctx.HTML(http.StatusOK, "isilon-export.gohtml", TriggerGETParams{
		ID:                  vxID,
		Title:               title,
		Filenames:           filenames,
		Languages:           s.languages,
		SelectedAudioSource: selectedAudioSource,
		AudioSources:        vidispine.ExportAudioSources.Values(),
		Resolutions:         resolutions,
		Ratio:               ratioString,
	})
}

func (s *TriggerServer) isilonExportPOST(ctx *gin.Context) {
	vxID := ctx.Query("id")

	queue := getQueue()
	workflowOptions := client.StartWorkflowOptions{
		TaskQueue: queue,
	}

	if os.Getenv("DEBUG") == "" {
		workflowOptions.SearchAttributes = map[string]any{
			"CustomStringField": vxID,
		}
	}

	resolutionIndex := bccmUtils.AsInt(ctx.PostForm("resolutions"))
	vsResolutions, err := s.vidispine.GetResolutions(vxID)
	if err != nil {
		renderErrorPage(ctx, http.StatusInternalServerError, err)
		return
	}

	selectedResolution := vsResolutions[resolutionIndex]

	spew.Dump(ctx.PostForm("exportFormat"))

	params := export.IsilonExportParams{
		VXID:          vxID,
		WatermarkPath: ctx.PostForm("watermarkPath"),
		AudioSource:   ctx.PostForm("audioSource"),
		Language:      ctx.PostForm("language"),
		Resolution:    utils.Resolution{Width: selectedResolution.Width, Height: selectedResolution.Height},
		ExportFormat:  ctx.PostForm("exportFormat"),
	}

	var wfID string
	workflowOptions.ID = params.VXID + "-" + shortid.MustGenerate()
	res, err := s.wfClient.ExecuteWorkflow(ctx, workflowOptions, export.IsilonExport, params)
	if err != nil {
		renderErrorPage(ctx, http.StatusInternalServerError, err)
		return
	}

	wfID = res.GetID()

	meta, err := s.vidispine.GetMetadata(vxID)
	if err != nil {
		renderErrorPage(ctx, http.StatusInternalServerError, err)
		return
	}

	ctx.HTML(http.StatusOK, "success.gohtml", gin.H{
		"WorkflowID": wfID,
		"Title":      meta.Get(vscommon.FieldTitle, ""),
	})
}
