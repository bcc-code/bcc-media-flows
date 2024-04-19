package vb_export

import (
	"fmt"
	"github.com/bcc-code/bcc-media-flows/services/telegram"
	"os"
	"strings"
	"time"

	"github.com/ansel1/merry/v2"
	"github.com/bcc-code/bcc-media-flows/activities"
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
	DestinationAbekas = Destination{Value: "abekas"}
	DestinationBStage = Destination{Value: "b-stage"}
	DestinationGfx    = Destination{Value: "gfx"}
	DestinationHippo  = Destination{Value: "hippo"}
	Destinations      = enum.New(
		DestinationAbekas,
		DestinationBStage,
		DestinationGfx,
		DestinationHippo,
	)
	deliveryFolder = paths.New(paths.BrunstadDrive, "/Delivery/FraMB/")
)

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
	Destinations     []string
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
	OriginalFilenameWithoutExt string
	SubtitleFile               *paths.Path
	SubtitleStyle              *paths.Path
	TempDir                    paths.Path
	OutputDir                  paths.Path
	AnalyzeResult              ffmpeg.StreamInfo
}

var subtitleStyleBase = os.Getenv("SUBTITLE_STYLES_DIR")

func VBExport(ctx workflow.Context, params VBExportParams) ([]wfutils.ResultOrError[VBExportResult], error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting VBExport")

	ctx = workflow.WithActivityOptions(ctx, wfutils.GetDefaultActivityOptions())

	if params.VXID == "" {
		return nil, fmt.Errorf("vxid is required")
	}

	var destinations []*Destination
	for _, dest := range params.Destinations {
		d := Destinations.Parse(dest)
		if d == nil {
			return nil, fmt.Errorf("invalid destination: %s", dest)
		}
		destinations = append(destinations, d)
	}

	var errs []error
	wfutils.NotifyTelegramChannel(ctx, telegram.ChatOslofjord, fmt.Sprintf("🟦 VB Export of %s started.\nDestination(s): %s\n\nRunID: %s", params.VXID, strings.Join(params.Destinations, ", "), workflow.GetInfo(ctx).OriginalRunID))

	shapes, err := avidispine.GetClient().GetShapes(params.VXID)
	if err != nil {
		return nil, err
	}

	logger.Info("Retrieved data from vidispine")

	if len(shapes.Shape) == 0 {
		return nil, fmt.Errorf("no clips found for VXID %s", params.VXID)
	}

	videoShape := shapes.GetShape("original")
	if videoShape == nil {
		return nil, fmt.Errorf("no original shape found for item %s", params.VXID)
	}

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
	originalFilenameWithoutExt := videoFilePath.Base()[0 : len(videoFilePath.Base())-len(videoFilePath.Ext())]
	var analyzeResult *ffmpeg.StreamInfo
	err = wfutils.Execute(ctx, activities.Audio.AnalyzeFile, activities.AnalyzeFileParams{
		FilePath: videoFilePath,
	}).Get(ctx, &analyzeResult)
	if err != nil {
		return nil, err
	}

	destinationsWithAudioOutput := lo.Filter(destinations, func(dest *Destination, _ int) bool {
		return *dest != DestinationHippo
	})

	if len(destinationsWithAudioOutput) > 0 && analyzeResult.HasAudio && !strings.Contains(videoFilePath.Base(), "_KLICK") {
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
		subtitleStylePath, err := paths.Parse(subtitleStyleBase + params.SubtitleStyle)
		if err != nil {
			return nil, err
		}
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
	for _, dest := range destinations {
		childParams := VBExportChildWorkflowParams{
			ParentParams:               params,
			OriginalFilenameWithoutExt: originalFilenameWithoutExt,
			InputFile:                  videoFilePath,
			SubtitleFile:               subtitleFile,
			SubtitleStyle:              subtitleStyle,
			TempDir:                    tempDir,
			OutputDir:                  outputDir.Append(dest.Value),
			RunID:                      workflow.GetInfo(ctx).OriginalRunID,
			AnalyzeResult:              *analyzeResult,
		}

		var w interface{}
		switch *dest {
		case DestinationAbekas:
			w = VBExportToAbekas
		case DestinationBStage:
			w = VBExportToBStage
		case DestinationGfx:
			w = VBExportToGfx
		case DestinationHippo:
			w = VBExportToHippo

		default:
			return nil, fmt.Errorf("destination not implemented: %s", dest)
		}

		err = wfutils.CreateFolder(ctx, childParams.OutputDir)
		if err != nil {
			return nil, err
		}

		ctx = workflow.WithChildOptions(ctx, wfutils.GetVXDefaultWorkflowOptions(params.VXID))
		future := workflow.ExecuteChildWorkflow(ctx, w, childParams)
		if err != nil {
			return nil, err
		}
		resultFutures = append(resultFutures, future)
	}

	var results []wfutils.ResultOrError[VBExportResult]
	for _, future := range resultFutures {
		var result *VBExportResult
		err = future.Get(ctx, &result)
		results = append(results, wfutils.ResultOrError[VBExportResult]{
			Result: result,
			Error:  err,
		})
		if err != nil {
			errs = append(errs, err)
			wfutils.NotifyTelegramChannel(ctx, telegram.ChatOslofjord, fmt.Sprintf("🟥 VB Export of %s failed: ```%s```", params.VXID, err.Error()))
			if err != nil {
				errs = append(errs, err)
			}
		}
	}
	err = nil
	if len(errs) > 0 {
		err = merry.New(strings.Join(lo.Map(errs, func(err error, _ int) string {
			return err.Error()
		}), "\n"))
	}
	return results, err
}

func notifyExportDone(ctx workflow.Context, params VBExportChildWorkflowParams, flow string, tempExportPath paths.Path) {
	message := fmt.Sprintf("🟩 Export of `%s` finished.\nDestination: `%s`, Preview: `%s`", params.ParentParams.VXID, flow, tempExportPath.Local())
	wfutils.NotifyTelegramChannel(ctx, telegram.ChatOslofjord, message)
}
