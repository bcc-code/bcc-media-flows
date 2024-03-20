package vb_export

import (
	"fmt"
	"strings"

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

type VBExportParams struct {
	VXID             string
	Destinations     []string
	SubtitleShapeTag string
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
	TempDir                    paths.Path
	OutputDir                  paths.Path
	AnalyzeResult              ffmpeg.StreamInfo
}

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
	err := wfutils.NotifyTelegramChannel(ctx, fmt.Sprintf("VB Export of %s started.\n\nRunID: %s", params.VXID, workflow.GetInfo(ctx).OriginalRunID))
	if err != nil {
		errs = append(errs, err)
	}

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

	if len(destinationsWithAudioOutput) > 0 && analyzeResult.HasAudio {
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
	if params.SubtitleShapeTag != "" {
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

		err = wfutils.NotifyTelegramChannel(ctx, fmt.Sprintf("Exporting VB %s to %s", params.VXID, dest.Value))
		if err != nil {
			errs = append(errs, err)
		}
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
			err = wfutils.NotifyTelegramChannel(ctx, fmt.Sprintf("VB Export of %s failed: ```%s```", params.VXID, err.Error()))
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

func notifyExportDone(ctx workflow.Context, params VBExportChildWorkflowParams, flow string) {
	_ = notifyTelegramChannel(ctx, fmt.Sprintf("🟩 Export of `%s` finished.\nDestination: `%s`", params.ParentParams.VXID, flow))
}

func notifyTelegramChannel(ctx workflow.Context, message string) error {
	err := wfutils.NotifyTelegramChannel(ctx, message)
	logger := workflow.GetLogger(ctx)
	if err != nil {
		logger.Error("Failed to notify telegram channel", "error", err)
	}
	return err
}
