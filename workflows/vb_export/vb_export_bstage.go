package vb_export

import (
	"go.temporal.io/sdk/workflow"
)

/*
VBExportToBStage
# Requirements

Container: MOV/MXF
Video: 1080p50, ProRes 422
Audio: PCM, 48kHz, 24Bit
Audio loudness: -23 dB LUFS
Audio tracks:
- Stream1, Track 1: PGM left
- Stream1, Track 2: PGM right
- Stream1, Track 3-16: Timecode/Multitrack Audio (optional)
*/
func VBExportToBStage(ctx workflow.Context, params VBExportChildWorkflowParams) (*VBExportResult, error) {
	return runVBExportChild(ctx, params, vbExportDestination{
		flow:       "bstage",
		folder:     "B-Stage",
		outputDir:  "b-stage_output",
		imageAware: true,
		transcode:  proRes{interlace: false, alpha: false}.transcode,
	})
}
