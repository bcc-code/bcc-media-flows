package main

import (
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/bcc-code/bcc-media-flows/services/vidispine/vsapi"
	"github.com/bcc-code/bcc-media-flows/services/vidispine/vscommon"
	"github.com/bcc-code/bcc-media-flows/workflows/vb_export"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.temporal.io/sdk/client"
)

type VBTriggerGETParams struct {
	Title          string
	Destinations   []string
	SubtitleShapes []string
	SubtitleStyles []string
}

var subtitleStylesDir = os.Getenv("SUBTITLE_STYLES_DIR")

func (s *TriggerServer) vbExportGET(ctx *gin.Context) {
	vxID := ctx.Query("id")
	meta, err := s.vidispine.GetMetadata(vxID)
	if err != nil {
		renderErrorPage(ctx, http.StatusInternalServerError, err)
		return
	}
	shapes, err := s.vidispine.GetShapes(vxID)
	if err != nil {
		renderErrorPage(ctx, http.StatusInternalServerError, err)
		return
	}

	var subtitleShapes []string
	for _, shape := range shapes.Shape {
		for _, tag := range shape.Tag {
			if strings.HasPrefix(tag, "sub_") && strings.HasSuffix(tag, "_srt") {
				subtitleShapes = append(subtitleShapes, tag)
			}
		}
	}

	clips := meta.SplitByClips()
	title := clips[vsapi.OriginalClip].Get(vscommon.FieldTitle, "")

	subStyles, err := getFilenames(subtitleStylesDir)
	if err != nil {
		log.Print(err)
		renderErrorPage(ctx, http.StatusInternalServerError, err)
		return
	}

	ctx.HTML(http.StatusOK, "vb-export.gohtml", VBTriggerGETParams{
		Title:          title,
		Destinations:   vb_export.Destinations.Values(),
		SubtitleShapes: subtitleShapes,
		SubtitleStyles: subStyles,
	})
}

func (s *TriggerServer) vbExportPOST(ctx *gin.Context) {
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

	params := vb_export.VBExportParams{
		VXID:             vxID,
		Destinations:     ctx.PostFormArray("destinations[]"),
		SubtitleShapeTag: ctx.PostForm("subtitleShape"),
		SubtitleStyle:    ctx.PostForm("subtitleStyle"),
	}

	var wfID string
	workflowOptions.ID = uuid.NewString()
	res, err := s.wfClient.ExecuteWorkflow(ctx, workflowOptions, vb_export.VBExport, params)

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
