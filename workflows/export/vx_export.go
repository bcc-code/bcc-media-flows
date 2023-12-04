package export

import (
	"fmt"
	"strings"
	"time"

	"github.com/ansel1/merry/v2"
	"github.com/bcc-code/bccm-flows/paths"
	"github.com/orsinium-labs/enum"
	"github.com/samber/lo"

	avidispine "github.com/bcc-code/bccm-flows/activities/vidispine"
	"github.com/bcc-code/bccm-flows/services/vidispine"
	"github.com/bcc-code/bccm-flows/utils/workflows"
	"go.temporal.io/sdk/workflow"
)

type AssetExportDestination enum.Member[string]

var (
	AssetExportDestinationPlayout = AssetExportDestination{Value: "playout"}
	AssetExportDestinationVOD     = AssetExportDestination{Value: "vod"}
	AssetExportDestinationBMM     = AssetExportDestination{Value: "bmm"}
	AssetExportDestinationIsilon  = AssetExportDestination{Value: "isilon"}
	AssetExportDestinations       = enum.New(
		AssetExportDestinationPlayout,
		AssetExportDestinationVOD,
		AssetExportDestinationBMM,
		AssetExportDestinationIsilon,
	)
)

type VXExportParams struct {
	VXID          string
	WithFiles     bool
	WithChapters  bool
	WatermarkPath string
	Destinations  []string
	AudioSource   string
	Languages     []string
	Subclip       string
}

type VXExportResult struct {
	ID           string `json:"id"`
	Title        string `json:"title"`
	Duration     string `json:"duration"`
	SmilFile     string `json:"smil_file"`
	ChaptersFile string `json:"chapters_file"`
}

type VXExportChildWorkflowParams struct {
	RunID        string
	ParentParams VXExportParams       `json:"parent_params"`
	ExportData   vidispine.ExportData `json:"export_data"`
	MergeResult  MergeExportDataResult
	TempDir      paths.Path
	OutputDir    paths.Path
	Upload       bool
}

func formatSecondsToTimestamp(seconds float64) string {
	hours := int(seconds / 3600)
	seconds -= float64(hours * 3600)

	minutes := int(seconds / 60)
	seconds -= float64(minutes * 60)

	secondsInt := int(seconds)

	return fmt.Sprintf("%02d:%02d:%02d:00", hours, minutes, secondsInt)
}

func VXExport(ctx workflow.Context, params VXExportParams) ([]wfutils.ResultOrError[VXExportResult], error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting VXExport")

	options := wfutils.GetDefaultActivityOptions()
	ctx = workflow.WithActivityOptions(ctx, options)

	var destinations []*AssetExportDestination
	for _, dest := range params.Destinations {
		d := AssetExportDestinations.Parse(dest)
		if d == nil {
			return nil, fmt.Errorf("invalid destination: %s", dest)
		}
		destinations = append(destinations, d)
	}

	var errs []error
	err := wfutils.NotifyTelegramChannel(ctx, fmt.Sprintf("Export of %s started.\n\nRunID: %s", params.VXID, workflow.GetInfo(ctx).OriginalRunID))
	if err != nil {
		errs = append(errs, err)
	}

	var data *vidispine.ExportData
	err = workflow.ExecuteActivity(ctx, avidispine.GetExportDataActivity, avidispine.GetExportDataParams{
		VXID:        params.VXID,
		Languages:   params.Languages,
		AudioSource: params.AudioSource,
		Subclip:     params.Subclip,
	}).Get(ctx, &data)
	if err != nil {
		return nil, err
	}

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

	ctx = workflow.WithChildOptions(ctx, wfutils.GetDefaultWorkflowOptions())

	bmmOnly := len(params.Destinations) == 1 && params.Destinations[0] == AssetExportDestinationBMM.Value

	var mergeResult MergeExportDataResult
	err = workflow.ExecuteChildWorkflow(ctx, MergeExportData, MergeExportDataParams{
		ExportData:    data,
		TempDir:       tempDir,
		SubtitlesDir:  subtitlesOutputDir,
		MakeVideo:     !bmmOnly,
		MakeAudio:     true,
		MakeSubtitles: true,
		Languages:     params.Languages,
	}).Get(ctx, &mergeResult)
	if err != nil {
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
			ParentParams: params,
			ExportData:   *data,
			MergeResult:  mergeResult,
			TempDir:      tempDir,
			OutputDir:    outputDir.Append(dest.Value),
			RunID:        workflow.GetInfo(ctx).OriginalRunID,
			Upload:       true,
		}

		var w interface{}
		switch *dest {
		case AssetExportDestinationIsilon:
			if hasDestination(AssetExportDestinationVOD) {
				// this is just a subflow of VOD
				continue
			}
			childParams.Upload = false
			fallthrough
		case AssetExportDestinationVOD:
			w = VXExportToVOD
			if hasDestination(AssetExportDestinationIsilon) {
				date := time.Now()
				id := workflow.GetInfo(ctx).OriginalRunID
				childParams.OutputDir = paths.Path{
					Drive: paths.IsilonDrive,
					Path:  fmt.Sprintf("Export/%s/%s", date.Format("2006-01"), data.SafeTitle+"-"+id[0:8]),
				}
			}
		case AssetExportDestinationPlayout:
			w = VXExportToPlayout
		case AssetExportDestinationBMM:
			w = VXExportToBMM
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

		err = wfutils.NotifyTelegramChannel(ctx, fmt.Sprintf("Exporting %s to %s", childParams.ExportData.Title, dest.Value))
		if err != nil {
			errs = append(errs, err)
		}
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
			err = wfutils.NotifyTelegramChannel(ctx, fmt.Sprintf("Export of %s failed: %s", params.VXID, err.Error()))
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
