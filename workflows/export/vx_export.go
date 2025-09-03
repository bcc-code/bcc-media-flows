package export

import (
	"fmt"
	"strings"

	"github.com/bcc-code/bcc-media-flows/utils"

	"github.com/ansel1/merry/v2"
	avidispine "github.com/bcc-code/bcc-media-flows/activities/vidispine"
	"github.com/bcc-code/bcc-media-flows/paths"
	"github.com/bcc-code/bcc-media-flows/services/telegram"
	"github.com/bcc-code/bcc-media-flows/services/vidispine"
	wfutils "github.com/bcc-code/bcc-media-flows/utils/workflows"
	"github.com/orsinium-labs/enum"
	"github.com/samber/lo"
	"go.temporal.io/sdk/workflow"
)

type AssetExportDestination enum.Member[string]

var (
	AssetExportDestinationPlayout        = AssetExportDestination{Value: "playout"}
	AssetExportDestinationVOD            = AssetExportDestination{Value: "vod"}
	AssetExportDestinationBMM            = AssetExportDestination{Value: "bmm"}
	AssetExportDestinationBMMIntegration = AssetExportDestination{Value: "bmm-integration"}
	AssetExportDestinationIsilon         = AssetExportDestination{Value: "isilon"}
	AssetExportDestinations              = enum.New(
		AssetExportDestinationPlayout,
		AssetExportDestinationVOD,
		AssetExportDestinationBMM,
		AssetExportDestinationBMMIntegration,
		AssetExportDestinationIsilon,
	)
)

type VXExportParams struct {
	VXID                      string
	WithChapters              bool
	WatermarkPath             string
	Destinations              []string `jsonschema:"enum=vod,enum=playout,enum=bmm,enum=bmm-integration,enum=isilon"`
	AudioSource               string
	Languages                 []string
	Subclip                   string
	IgnoreSilence             bool
	Resolutions               []utils.Resolution
	SubsAllowAI               bool
	ForceReplaceTranscription bool
}

type VXExportResult struct {
	ID           string `json:"id"`
	Title        string `json:"title"`
	Duration     string `json:"duration"`
	SmilFile     string `json:"smil_file"`
	ChaptersFile string `json:"chapters_file"`
}

type VXExportChildWorkflowParams struct {
	RunID                     string
	ParentParams              VXExportParams       `json:"parent_params"`
	ExportData                vidispine.ExportData `json:"export_data"`
	MergeResult               MergeExportDataResult
	TempDir                   paths.Path
	OutputDir                 paths.Path
	Upload                    bool
	ExportDestination         AssetExportDestination
	ForceReplaceTranscription bool
}

// VXExport is the main workflow for exporting assets from vidispine
// It will create a child workflow for each destination
func VXExport(ctx workflow.Context, params VXExportParams) ([]wfutils.ResultOrError[VXExportResult], error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting VXExport")

	ctx = workflow.WithActivityOptions(ctx, wfutils.GetDefaultActivityOptions())

	telegramChat := telegram.ChatVOD
	if len(params.Destinations) == 1 &&
		(params.Destinations[0] == AssetExportDestinationBMM.Value || params.Destinations[0] == AssetExportDestinationBMMIntegration.Value) {
		telegramChat = telegram.ChatBMM
	}

	var destinations []*AssetExportDestination
	for _, dest := range params.Destinations {
		d := AssetExportDestinations.Parse(dest)
		if d == nil {
			return nil, fmt.Errorf("invalid destination: %s", dest)
		}
		destinations = append(destinations, d)
	}

	var errs []error
	var data *vidispine.ExportData
	err := wfutils.Execute(ctx, avidispine.Vidispine.GetExportDataActivity, avidispine.GetExportDataParams{
		VXID:        params.VXID,
		Languages:   params.Languages,
		AudioSource: params.AudioSource,
		Subclip:     params.Subclip,
		SubsAllowAI: params.SubsAllowAI,
	}).Get(ctx, &data)
	if err != nil {
		return nil, err
	}

	wfutils.SendTelegramText(ctx,
		telegramChat,
		fmt.Sprintf(
			"🟦 Export of `%s` started.\nTitle: `%s`\nDestinations: `%s`\n\nRunID: `%s`",
			params.VXID,
			data.Title,
			strings.Join(params.Destinations, ", "),
			workflow.GetInfo(ctx).OriginalRunID,
		),
	)

	logger.Info("Retrieved data from vidispine")

	tempDir, err := wfutils.GetWorkflowTempFolder(ctx)
	if err != nil {
		return nil, err
	}

	outputDir := tempDir.Append("output")
	err = wfutils.CreateFolder(ctx, outputDir)
	if err != nil {
		return nil, err
	}

	subtitlesOutputDir := outputDir.Append("subtitles")
	err = wfutils.CreateFolder(ctx, subtitlesOutputDir)
	if err != nil {
		return nil, err
	}

	ctx = workflow.WithChildOptions(ctx, wfutils.GetVXDefaultWorkflowOptions(params.VXID))

	bmmOnly := len(params.Destinations) == 1 && (params.Destinations[0] == AssetExportDestinationBMM.Value || params.Destinations[0] == AssetExportDestinationBMMIntegration.Value)

	audioOnly := (len(data.Clips) > 0 && data.Clips[0].VideoFile == "") || bmmOnly

	var mergeResult MergeExportDataResult
	err = workflow.ExecuteChildWorkflow(ctx, MergeExportData, MergeExportDataParams{
		ExportData:       data,
		TempDir:          tempDir,
		SubtitlesDir:     subtitlesOutputDir,
		MakeVideo:        !audioOnly,
		MakeAudio:        true,
		MakeSubtitles:    true,
		MakeTranscript:   true,
		Languages:        params.Languages,
		OriginalLanguage: data.OriginalLanguage,
	}).Get(ctx, &mergeResult)
	if err != nil {
		wfutils.SendTelegramText(ctx, telegramChat, fmt.Sprintf("🟥 Export of `%s` failed:\n```\n%s\n```", params.VXID, err.Error()))
		return nil, err
	}

	hasDestination := func(d AssetExportDestination) bool {
		return lo.SomeBy(destinations, func(dest *AssetExportDestination) bool {
			return *dest == d
		})
	}

	// Destination branching:  VOD, playout, bmm, etc.
	var resultFutures []workflow.Future
	for _, dest := range destinations {
		childParams := VXExportChildWorkflowParams{
			ParentParams:              params,
			ExportData:                *data,
			MergeResult:               mergeResult,
			TempDir:                   tempDir,
			OutputDir:                 outputDir.Append(dest.Value),
			RunID:                     workflow.GetInfo(ctx).OriginalRunID,
			Upload:                    true,
			ExportDestination:         *dest,
			ForceReplaceTranscription: params.ForceReplaceTranscription,
		}

		var w interface{}
		switch *dest {
		case AssetExportDestinationIsilon:
			if hasDestination(AssetExportDestinationVOD) {
				// this is just a subflow of VOD
				continue
			}
			childParams.Upload = false
			date := workflow.Now(ctx)
			id := workflow.GetInfo(ctx).OriginalRunID
			childParams.OutputDir = paths.Path{
				Drive: paths.IsilonDrive,
				Path:  fmt.Sprintf("Export/%s/%s", date.Format("2006-01"), data.SafeTitle+"-"+id[0:8]),
			}
			fallthrough
		case AssetExportDestinationVOD:
			w = VXExportToVOD
		case AssetExportDestinationPlayout:
			w = VXExportToPlayout
		case AssetExportDestinationBMM, AssetExportDestinationBMMIntegration:
			w = VXExportToBMM
		default:
			return nil, fmt.Errorf("destination not implemented: %s", dest)
		}

		err = wfutils.CreateFolder(ctx, childParams.OutputDir)
		if err != nil {
			return nil, err
		}

		ctx = workflow.WithChildOptions(ctx, wfutils.GetVXDefaultWorkflowOptions(params.VXID))
		future := workflow.ExecuteChildWorkflow(ctx, w, childParams)
		resultFutures = append(resultFutures, future)
	}

	var results []wfutils.ResultOrError[VXExportResult]
	for _, future := range resultFutures {
		var result *VXExportResult
		err = future.Get(ctx, &result)
		results = append(results, wfutils.ResultOrError[VXExportResult]{
			Result: result,
			Error:  err,
		})
		if err != nil {
			errs = append(errs, err)
			wfutils.SendTelegramText(ctx, telegramChat, fmt.Sprintf("🟥 Export of `%s` failed:\n```\n%s\n```", params.VXID, err.Error()))
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
