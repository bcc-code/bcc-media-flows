package vb_export

import (
	"fmt"
	"strings"

	"github.com/ansel1/merry/v2"
	"github.com/bcc-code/bccm-flows/activities"
	"github.com/bcc-code/bccm-flows/paths"
	"github.com/orsinium-labs/enum"
	"github.com/samber/lo"

	avidispine "github.com/bcc-code/bccm-flows/activities/vidispine"
	"github.com/bcc-code/bccm-flows/services/ffmpeg"
	"github.com/bcc-code/bccm-flows/services/vidispine"
	wfutils "github.com/bcc-code/bccm-flows/utils/workflows"
	"go.temporal.io/sdk/workflow"
)

type VBExportDestination enum.Member[string]

var (
	VBExportDestinationAbekas = VBExportDestination{Value: "abekas"}
	VBExportDestinationBStage = VBExportDestination{Value: "bstage"}
	VBExportDestinationGfx    = VBExportDestination{Value: "gfx"}
	VBExportDestinationHippo  = VBExportDestination{Value: "hippo"}
	VBExportDestinations      = enum.New(
		VBExportDestinationAbekas,
		VBExportDestinationBStage,
		VBExportDestinationGfx,
		VBExportDestinationHippo,
	)
)

type VBExportParams struct {
	VXID         string
	Destinations []string
}

type VBExportResult struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Duration string `json:"duration"`
}

type VBExportChildWorkflowParams struct {
	RunID               string
	ParentParams        VBExportParams       `json:"parent_params"`
	ExportData          vidispine.ExportData `json:"export_data"`
	InputFile           paths.Path
	SubtitleFile        *paths.Path
	NormalizedAudioFile *paths.Path
	TempDir             paths.Path
	OutputDir           paths.Path
	AnalyzeResult       ffmpeg.StreamInfo
}

func VBExport(ctx workflow.Context, params VBExportParams) ([]wfutils.ResultOrError[VBExportResult], error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting VBExport")

	options := wfutils.GetDefaultActivityOptions()
	ctx = workflow.WithActivityOptions(ctx, options)

	var destinations []*VBExportDestination
	for _, dest := range params.Destinations {
		d := VBExportDestinations.Parse(dest)
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

	var data *vidispine.ExportData
	err = workflow.ExecuteActivity(ctx, avidispine.GetExportDataActivity, avidispine.GetExportDataParams{
		VXID:        params.VXID,
		AudioSource: vidispine.ExportAudioSourceEmbedded.Value,
	}).Get(ctx, &data)
	if err != nil {
		return nil, err
	}

	logger.Info("Retrieved data from vidispine")

	if len(data.Clips) == 0 {
		return nil, fmt.Errorf("no clips found for VXID %s", params.VXID)
	}
	clip := data.Clips[0]

	tempDir, err := wfutils.GetWorkflowTempFolder(ctx)
	if err != nil {
		return nil, err
	}

	outputDir := tempDir.Append("output")
	err = wfutils.CreateFolder(ctx, outputDir)
	if err != nil {
		return nil, err
	}

	ctx = workflow.WithChildOptions(ctx, wfutils.GetDefaultWorkflowOptions())

	videoFilePath := paths.MustParse(clip.VideoFile)
	var analyzeResult *ffmpeg.StreamInfo
	err = wfutils.ExecuteWithQueue(ctx, activities.AnalyzeFile, activities.AnalyzeFileParams{
		FilePath: videoFilePath,
	}).Get(ctx, &analyzeResult)
	if err != nil {
		return nil, err
	}

	destinationsWithAudioOutput := lo.Filter(destinations, func(dest *VBExportDestination, _ int) bool {
		return *dest != VBExportDestinationHippo
	})

	var normalizedAudioFile *paths.Path
	if len(destinationsWithAudioOutput) > 0 && analyzeResult.HasAudio {
		// Normalize audio
		var normalizeAudioResult *activities.NormalizeAudioResult
		err = wfutils.ExecuteWithQueue(ctx, activities.NormalizeAudioActivity, activities.NormalizeAudioParams{
			FilePath:              videoFilePath,
			TargetLUFS:            -23,
			PerformOutputAnalysis: true,
			OutputPath:            tempDir,
		}).Get(ctx, &normalizeAudioResult)
		if err != nil {
			return nil, err
		}
	} else {
		logger.Info("No destinations for audio, skipping normalize")
	}

	var subtitleFile *paths.Path
outer:
	for _, v := range data.Clips {
		if len(v.SubtitleFiles) > 0 {
			for _, v := range v.SubtitleFiles {
				path, err := paths.Parse(v)
				if err == nil {
					subtitleFile = &path
				}
				break outer
			}
		}
		break
	}

	ctx = workflow.WithChildOptions(ctx, wfutils.GetDefaultWorkflowOptions())

	var resultFutures []workflow.Future
	for _, dest := range destinations {
		childParams := VBExportChildWorkflowParams{
			ParentParams:        params,
			ExportData:          *data,
			InputFile:           videoFilePath,
			SubtitleFile:        subtitleFile,
			NormalizedAudioFile: normalizedAudioFile,
			TempDir:             tempDir,
			OutputDir:           outputDir.Append(dest.Value),
			RunID:               workflow.GetInfo(ctx).OriginalRunID,
			AnalyzeResult:       *analyzeResult,
		}

		var w interface{}
		switch *dest {
		case VBExportDestinationAbekas:
			w = VBExportToAbekas
		case VBExportDestinationBStage:
			w = VBExportToBStage
		case VBExportDestinationGfx:
			w = VBExportToGfx
		case VBExportDestinationHippo:
			w = VBExportToHippo

		default:
			return nil, fmt.Errorf("destination not implemented: %s", dest)
		}

		err = wfutils.CreateFolder(ctx, childParams.OutputDir)
		if err != nil {
			return nil, err
		}

		ctx = workflow.WithChildOptions(ctx, wfutils.GetDefaultWorkflowOptions())
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
			err = wfutils.NotifyTelegramChannel(ctx, fmt.Sprintf("VB Export of %s failed: %s", params.VXID, err.Error()))
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
