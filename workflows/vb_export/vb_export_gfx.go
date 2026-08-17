package vb_export

import (
	"go.temporal.io/sdk/workflow"
)

/*
VBExportToGfx
# Requirements

Container: MOV/MXF
Video: 1080i50, ProRes 4444
Audio: PCM, 48kHz, 24Bit
Audio loudness: -23 dB LUFS
Audio tracks:
- Stream1, Track 1: PGM left (optional)
- Stream1, Track 2: PGM right (optional)
- Stream1, Track 3-16: Timecode/Multitrack Audio (optional)
*/
func VBExportToGfx(ctx workflow.Context, params VBExportChildWorkflowParams) (*VBExportResult, error) {
	return runVBExportChild(ctx, params, vbExportDestination{
		destination: DestinationGfx,
		imageAware:  true,
		transcode:   proRes{interlace: true, alpha: true}.transcode,
	})
}
