package vb_export

import (
	"errors"
	"fmt"
	"github.com/bcc-code/bcc-media-flows/services/vidispine/vsapi"
	"path/filepath"
	"strings"
	"time"

	"github.com/bcc-code/bcc-media-flows/services/telegram"

	"github.com/bcc-code/bcc-media-flows/activities"
	"github.com/bcc-code/bcc-media-flows/environment"
	"github.com/bcc-code/bcc-media-flows/internal/enumjson"
	"github.com/bcc-code/bcc-media-flows/paths"
	"github.com/orsinium-labs/enum"
	"github.com/samber/lo"

	avidispine "github.com/bcc-code/bcc-media-flows/activities/vidispine"
	"github.com/bcc-code/bcc-media-flows/services/ffmpeg"
	wfutils "github.com/bcc-code/bcc-media-flows/utils/workflows"
	"go.temporal.io/sdk/workflow"
)

type Destination enum.Member[string]

var (
	DestinationAbekas    = Destination{Value: "abekas"}
	DestinationRawAbekas = Destination{Value: "raw-abekas"}
	DestinationBStage    = Destination{Value: "b-stage"}
	DestinationHyperdeck = Destination{Value: "hyperdeck"}
	DestinationGfx       = Destination{Value: "gfx"}
	DestinationHippoV2   = Destination{Value: "hippo_v2"}
	DestinationHippoHap  = Destination{Value: "hippo_hap"}
	DestinationDubbing   = Destination{Value: "dubbing"}
	DestinationXDCAM     = Destination{Value: "xdcam"}
	DestinationCasparCG  = Destination{Value: "caspar-cg"}
	Destinations         = enum.New(
		DestinationAbekas,
		DestinationRawAbekas,
		DestinationBStage,
		DestinationGfx,
		DestinationHippoV2,
		DestinationHippoHap,
		DestinationDubbing,
		DestinationHyperdeck,
		DestinationXDCAM,
		DestinationCasparCG,
	)
	deliveryFolder        = paths.New(paths.BrunstadDrive, "/Delivery/FraMB/")
	ErrUnknownDestination = errors.New("unknown VB export destination")
)

func (d Destination) String() string {
	return d.Value
}

// MarshalJSON writes the bare destination name, so payloads look the same as
// they did while Destinations was a []string.
//
//goland:noinspection GoMixedReceiverTypes
func (d Destination) MarshalJSON() ([]byte, error) {
	return enumjson.Marshal(d)
}

//goland:noinspection GoMixedReceiverTypes
func (d *Destination) UnmarshalJSON(data []byte) error {
	return enumjson.UnmarshalStrict(data, Destinations, d, ErrUnknownDestination)
}

var destinationDescriptions = map[Destination]string{
	DestinationAbekas:    "Videoavspilling i bussen",
	DestinationRawAbekas: "Videoavspilling i bussen (originalfil overføres)",
	DestinationBStage:    "Avspilling utenfor plenumsalen (festsalen, arenaen, etc)",
	DestinationGfx:       "",
	DestinationHippoV2:   "For LED (stor fil)",
	DestinationHippoHap:  "For LED (litt mindre fil)",
	DestinationDubbing:   "For multispråk avspilling",
	DestinationHyperdeck: "Brukes hvis spesifikt etterspurt",
	DestinationXDCAM:     "",
	DestinationCasparCG:  "Brukes hvis spesifikt etterspurt",
}

func (d Destination) Description() string {
	return destinationDescriptions[d]
}

// destinationFolders are the subfolders of deliveryFolder each destination is
// delivered to. Abekas has a second one, Abekas-WAV, for audio-only exports.
var destinationFolders = map[Destination]string{
	DestinationAbekas:    "Abekas-AVCI",
	DestinationRawAbekas: "Abekas-RAW",
	DestinationBStage:    "B-Stage",
	DestinationGfx:       "GFX",
	DestinationHippoV2:   "Hippo",
	DestinationHippoHap:  "Hippo",
	DestinationDubbing:   "Reaper-Wav",
	DestinationHyperdeck: "Hyperdeck-ProRes",
	DestinationXDCAM:     "XDCAM",
	DestinationCasparCG:  "CasparCG",
}

func (d Destination) DeliveryFolder() string {
	return destinationFolders[d]
}

// OutputDirName is the per-destination subfolder of the workflow's temp dir.
func (d Destination) OutputDirName() string {
	return d.Value + "_output"
}

var destinationWorkflows = map[Destination]any{
	DestinationAbekas:    VBExportToAbekas,
	DestinationRawAbekas: VBExportToRawAbekas,
	DestinationBStage:    VBExportToBStage,
	DestinationGfx:       VBExportToGfx,
	DestinationHippoV2:   VBExportToHippoV2,
	DestinationHippoHap:  VBExportToHippoHap,
	DestinationDubbing:   VBExportToDubbing,
	DestinationHyperdeck: VBExportToHyperdeck,
	DestinationXDCAM:     VBExportToXDCAM,
	DestinationCasparCG:  VBExportToCasparCG,
}

var (
	rcloneNotificationOptions = &activities.TelegramNotificationOptions{
		ChatID:               telegram.ChatOslofjord,
		NotificationInterval: time.Minute,
		StartNotification:    true,
		EndNotification:      true,
	}
)

type VBExportParams struct {
	VXID             string
	Destinations     []Destination
	SubtitleShapeTag string
	SubtitleStyle    string
}

type VBExportResult struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Duration string `json:"duration"`
}

type VBExportChildWorkflowParams struct {
	RunID                      string
	ParentParams               VBExportParams `json:"parent_params"`
	InputFile                  paths.Path
	OriginalFile               paths.Path
	OriginalFilenameWithoutExt string
	SubtitleFile               *paths.Path
	SubtitleStyle              *paths.Path
	TempDir                    paths.Path
	OutputDir                  paths.Path
	AnalyzeResult              ffmpeg.StreamInfo
}

// subtitleStyleDir goes through SideEffect so a replay on a differently configured
// worker builds the same path.
func subtitleStyleDir(ctx workflow.Context) (string, error) {
	var dir string
	err := workflow.SideEffect(ctx, func(workflow.Context) any {
		return environment.Get().Paths.SubtitleStyles()
	}).Get(&dir)

	return dir, err
}

func VBExport(ctx workflow.Context, params VBExportParams) ([]wfutils.ResultOrError[VBExportResult], error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting VBExport")

	ctx = workflow.WithActivityOptions(ctx, wfutils.GetDefaultActivityOptions())

	if params.VXID == "" {
		return nil, errors.New("vxid is required")
	}

	for _, dest := range params.Destinations {
		if _, ok := destinationWorkflows[dest]; !ok {
			return nil, fmt.Errorf("%w: %s", ErrUnknownDestination, dest)
		}
	}

	shapes, err := wfutils.Execute(ctx, activities.Vidispine.GetShapes, avidispine.VXOnlyParam{
		VXID: params.VXID,
	}).Result(ctx)
	if err != nil {
		return nil, err
	}

	logger.Info("Retrieved data from vidispine")

	if len(shapes.Shape) == 0 {
		return nil, fmt.Errorf("no clips found for VXID %s", params.VXID)
	}

	videoShape := shapes.GetShape(vsapi.ShapeTagOriginal)
	if videoShape == nil {
		return nil, fmt.Errorf("no original shape found for item %s", params.VXID)
	}

	wfutils.SendTelegramText(ctx, telegram.ChatOslofjord,
		fmt.Sprintf("🟦 VB Export of %s - `%s` started.\nDestination(s): `%s`\n\nRunID: %s",
			params.VXID, filepath.Base(videoShape.GetPath()), strings.Join(lo.Map(params.Destinations, func(d Destination, _ int) string { return d.Value }), ", "), workflow.GetInfo(ctx).OriginalRunID,
		),
	)

	tempDir, err := wfutils.GetWorkflowTempFolder(ctx)
	if err != nil {
		return nil, err
	}

	outputDir := tempDir.Append("output")
	err = wfutils.CreateFolder(ctx, outputDir)
	if err != nil {
		return nil, err
	}

	videoFilePath := paths.MustParse(videoShape.GetPath())
	originalVideoFilePath := videoFilePath

	originalFilenameWithoutExt := videoFilePath.Base()[0 : len(videoFilePath.Base())-len(videoFilePath.Ext())]
	analyzeResult, err := wfutils.Execute(ctx, activities.Audio.AnalyzeFile, activities.AnalyzeFileParams{
		FilePath: videoFilePath,
	}).Result(ctx)
	if err != nil {
		return nil, err
	}

	destinationsWithAudioOutput := lo.Filter(params.Destinations, func(dest Destination, _ int) bool {
		return dest != DestinationCasparCG
	})

	if len(destinationsWithAudioOutput) > 0 && analyzeResult.HasAudio && len(analyzeResult.AudioStreams) <= 2 {
		normalizeAudioResult, err := wfutils.Execute(ctx, activities.Audio.NormalizeAudioActivity, activities.NormalizeAudioParams{
			FilePath:              videoFilePath,
			TargetLUFS:            -23,
			PerformOutputAnalysis: true,
			OutputPath:            tempDir,
		}).Result(ctx)
		if err != nil {
			return nil, err
		}
		videoFilePath = normalizeAudioResult.FilePath
	} else {
		logger.Info("No destinations for audio, skipping normalize")
	}

	var subtitleFile *paths.Path
	var subtitleStyle *paths.Path
	if params.SubtitleShapeTag != "" {
		styleDir, err := subtitleStyleDir(ctx)
		if err != nil {
			return nil, err
		}

		subtitleStylePath := paths.MustParse(styleDir + params.SubtitleStyle)
		subtitleStyle = &subtitleStylePath

	outer:
		for _, shape := range shapes.Shape {
			for _, tag := range shape.Tag {
				if tag == params.SubtitleShapeTag {
					path := paths.MustParse(shape.GetPath())
					subtitleFile = &path
					break outer
				}
			}
		}
	}

	var resultFutures []workflow.Future
	for _, dest := range params.Destinations {
		childParams := VBExportChildWorkflowParams{
			ParentParams:               params,
			OriginalFilenameWithoutExt: originalFilenameWithoutExt,
			InputFile:                  videoFilePath,
			OriginalFile:               originalVideoFilePath,
			SubtitleFile:               subtitleFile,
			SubtitleStyle:              subtitleStyle,
			TempDir:                    tempDir,
			OutputDir:                  outputDir.Append(dest.Value),
			RunID:                      workflow.GetInfo(ctx).OriginalRunID,
			AnalyzeResult:              *analyzeResult,
		}

		err = wfutils.CreateFolder(ctx, childParams.OutputDir)
		if err != nil {
			return nil, err
		}

		ctx = workflow.WithChildOptions(ctx, wfutils.GetVXDefaultWorkflowOptions(ctx, params.VXID))
		future := workflow.ExecuteChildWorkflow(ctx, destinationWorkflows[dest], childParams)
		resultFutures = append(resultFutures, future)
	}

	return wfutils.CollectChildResults[VBExportResult](ctx, resultFutures, func(err error) {
		wfutils.SendTelegramText(ctx, telegram.ChatOslofjord, fmt.Sprintf("🟥 VB Export of %s failed: ```%s```", params.VXID, err.Error()))
	})
}

func notifyExportDone(ctx workflow.Context, params VBExportChildWorkflowParams, destination Destination, tempExportPath paths.Path) {
	message := fmt.Sprintf("🟩 Export of `%s` finished.\nDestination: `%s`, Preview: `%s`", params.ParentParams.VXID, destination.Value, tempExportPath.Local())
	wfutils.SendTelegramText(ctx, telegram.ChatOslofjord, message)
}
