package main

import (
	"github.com/bcc-code/bcc-media-flows/environment"
	"log"
	"net/http"
	"strings"

	"github.com/bcc-code/bcc-media-flows/services/vidispine/vsapi"
	"github.com/bcc-code/bcc-media-flows/services/vidispine/vscommon"
	wfutils "github.com/bcc-code/bcc-media-flows/utils/workflows"
	"github.com/bcc-code/bcc-media-flows/workflows/vb_export"
	"github.com/gin-gonic/gin"
)

type VBTriggerGETParams struct {
	Title          string
	Destinations   []string
	SubtitleShapes []string
	SubtitleStyles []string
}

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

	subStyles, err := getFilenames(environment.Get().Paths.SubtitleStyles())
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

	workflowOptions := wfutils.NewWorkflowOptions(environment.GetQueue(), vxID, getTriggeredBy(ctx))

	params := vb_export.VBExportParams{
		VXID:             vxID,
		Destinations:     ctx.PostFormArray("destinations[]"),
		SubtitleShapeTag: ctx.PostForm("subtitleShape"),
		SubtitleStyle:    ctx.PostForm("subtitleStyle"),
	}

	var wfID string
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
