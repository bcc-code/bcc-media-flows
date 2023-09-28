package export

import (
	"fmt"
	"path/filepath"

	"github.com/orsinium-labs/enum"

	avidispine "github.com/bcc-code/bccm-flows/activities/vidispine"
	"github.com/bcc-code/bccm-flows/services/vidispine"
	"github.com/bcc-code/bccm-flows/utils/wfutils"
	"github.com/bcc-code/bccm-flows/workflows"
	"go.temporal.io/sdk/workflow"
)

type AssetExportDestination enum.Member[string]

var (
	AssetExportDestinationPlayout = AssetExportDestination{"playout"}
	AssetExportDestinationVOD     = AssetExportDestination{"vod"}
	AssetExportDestinationBMM     = AssetExportDestination{"bmm"}
	AssetExportDestinations       = enum.New(
		AssetExportDestinationPlayout,
		AssetExportDestinationVOD,
		AssetExportDestinationBMM,
	)
)

type VXExportParams struct {
	VXID          string
	WithFiles     bool
	WithChapters  bool
	WatermarkPath string
	Destinations  []string
}

type VXExportResult struct {
	ID           string `json:"id"`
	Title        string `json:"title"`
	Duration     string `json:"duration"`
	SmilFile     string `json:"smil_file"`
	ChaptersFile string `json:"chapters_file"`
}

type VXExportChildWorklowParams struct {
	ParentParams VXExportParams       `json:"parent_params"`
	ExportData   vidispine.ExportData `json:"export_data"`
	MergeResult  MergeExportDataResult
	TempDir      string
	OutputDir    string
}

func formatSecondsToTimestamp(seconds float64) string {
	hours := int(seconds / 3600)
	seconds -= float64(hours * 3600)

	minutes := int(seconds / 60)
	seconds -= float64(minutes * 60)

	secondsInt := int(seconds)

	return fmt.Sprintf("%02d:%02d:%02d:00", hours, minutes, secondsInt)
}

func VXExport(ctx workflow.Context, params VXExportParams) ([]workflows.ResultOrError[VXExportResult], error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting VXExport")

	options := workflows.GetDefaultActivityOptions()
	ctx = workflow.WithActivityOptions(ctx, options)

	var destinations []*AssetExportDestination
	for _, dest := range params.Destinations {
		d := AssetExportDestinations.Parse(dest)
		if d == nil {
			return nil, fmt.Errorf("invalid destination: %s", dest)
		}
		destinations = append(destinations, d)
	}

	var data *vidispine.ExportData
	err := workflow.ExecuteActivity(ctx, avidispine.GetExportDataActivity, avidispine.GetExportDataParams{
		VXID: params.VXID,
	}).Get(ctx, &data)
	if err != nil {
		return nil, err
	}

	logger.Info("Retrieved data from vidispine")

	tempDir, err := wfutils.GetWorkflowTempFolder(ctx)
	if err != nil {
		return nil, err
	}

	outputDir := filepath.Join(tempDir, "output")
	err = wfutils.CreateFolder(ctx, outputDir)
	if err != nil {
		return nil, err
	}

	ctx = workflow.WithChildOptions(ctx, workflows.GetDefaultWorkflowOptions())

	var mergeResult MergeExportDataResult
	err = workflow.ExecuteChildWorkflow(ctx, MergeExportData, MergeExportDataParams{
		ExportData:   data,
		TempDir:      tempDir,
		SubtitlesDir: outputDir,
	}).Get(ctx, &mergeResult)
	if err != nil {
		return nil, err
	}

	// Destination branching:  VOD, playout, bmm, etc.
	var resultFutures []workflow.Future
	for _, dest := range destinations {

		var w interface{}
		switch *dest {
		case AssetExportDestinationVOD:
			w = VXExportToVOD
		case AssetExportDestinationPlayout:
			w = VXExportToPlayout
		default:
			return nil, fmt.Errorf("destination not implemented: %s", dest)
		}

		ctx = workflow.WithChildOptions(ctx, workflows.GetDefaultWorkflowOptions())
		future := workflow.ExecuteChildWorkflow(ctx, w, VXExportChildWorklowParams{
			ParentParams: params,
			ExportData:   *data,
			MergeResult:  mergeResult,
			TempDir:      tempDir,
			OutputDir:    outputDir,
		})
		if err != nil {
			return nil, err
		}
		resultFutures = append(resultFutures, future)
	}

	results := []workflows.ResultOrError[VXExportResult]{}
	for _, future := range resultFutures {
		var result *VXExportResult
		err = future.Get(ctx, &result)
		results = append(results, workflows.ResultOrError[VXExportResult]{
			Result: result,
			Error:  err,
		})
	}

	return results, nil
}
