package miscworkflows

import (
	"encoding/csv"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/bcc-code/bcc-media-flows/activities"
	vsactivity "github.com/bcc-code/bcc-media-flows/activities/vidispine"
	"github.com/bcc-code/bcc-media-flows/common"
	"github.com/bcc-code/bcc-media-flows/paths"
	"github.com/bcc-code/bcc-media-flows/services/telegram"
	wfutils "github.com/bcc-code/bcc-media-flows/utils/workflows"
	"github.com/gocarina/gocsv"
	"go.temporal.io/sdk/workflow"
)

type MergeAndImportSubtitlesFromCSVParams struct {
	TargetVXID string
	CSVData    string
	Title      string
	Separator  string
}

func convertCSVTimestamp(timestamp string) (float64, error) {
	parts := strings.Split(timestamp, ":")
	if len(parts) != 4 {
		return 0, fmt.Errorf("invalid timestamp format: %s", timestamp)
	}

	// Parse the time components
	t, err := time.Parse("15:04:05", strings.Join(parts[:3], ":"))
	if err != nil {
		return 0, err
	}

	// Parse milliseconds
	ms, err := time.ParseDuration(parts[3] + "ms")
	if err != nil {
		return 0, err
	}

	return float64(t.Hour()*3600+t.Minute()*60+t.Second()) + ms.Seconds(), nil
}

func getSeparatorRune(s string) rune {
	for _, r := range s {
		return r
	}
	return ','
}

func MergeAndImportSubtitlesFromCSV(ctx workflow.Context, params MergeAndImportSubtitlesFromCSVParams) (bool, error) {

	logger := workflow.GetLogger(ctx)

	options := wfutils.GetDefaultActivityOptions()
	ctx = workflow.WithActivityOptions(ctx, options)

	logger.Info("Starting sub merge and import")
	wfutils.SendTelegramText(ctx, telegram.ChatOther, "🟦 Starting sub merge and import to VXID: "+params.TargetVXID)

	tempPath, _ := wfutils.GetWorkflowTempFolder(ctx)
	outputPath, _ := wfutils.GetWorkflowAuxOutputFolder(ctx)

	entries, err := parseSubMergeData([]byte(params.CSVData), getSeparatorRune(params.Separator))
	if err != nil {
		return false, err
	}

	mergeData := map[string]*common.MergeInput{}

	for _, entry := range entries {
		offset, err := convertCSVTimestamp(entry.TimecodeStr)
		if err != nil {
			return false, err
		}

		res, err := wfutils.Execute(ctx, activities.Util.GetSubtitlesActivity, activities.GetSubtitlesInput{
			SubtransID:        entry.SubtransID,
			Format:            "srt",
			ApprovedOnly:      false,
			DestinationFolder: tempPath,
		}).Result(ctx)

		for lang, sub := range res {

			if _, ok := mergeData[lang]; !ok {
				mergeData[lang] = &common.MergeInput{
					Title:     params.Title,
					WorkDir:   tempPath,
					OutputDir: outputPath,
				}
			}

			mergeData[lang].Items = append(mergeData[lang].Items, common.MergeInputItem{
				StartOffset: offset,
				Path:        sub,
			})
		}

		if err != nil {
			return false, err
		}
	}

	merged := map[string]paths.Path{}

	langs, err := wfutils.GetMapKeysSafely(ctx, mergeData)
	if err != nil {
		return false, err
	}

	for _, lang := range langs {
		merge := mergeData[lang]
		res, err := wfutils.Execute(ctx, activities.Audio.MergeSubtitlesByOffset, *merge).Result(ctx)
		if err != nil {
			return false, err
		}
		merged[lang] = res.Path
	}

	for _, lang := range langs {
		sub := merged[lang]
		lang := strings.ToLower(lang)

		jobRes := &vsactivity.JobResult{}
		err = wfutils.Execute(ctx, activities.Vidispine.ImportFileAsShapeActivity, vsactivity.ImportFileAsShapeParams{
			AssetID:  params.TargetVXID,
			FilePath: sub,
			ShapeTag: fmt.Sprintf("sub_%s_%s", lang, "srt"),
			Replace:  true,
		}).Get(ctx, jobRes)

		if jobRes.JobID == "" {
			logger.Info("No job created for importing subtitle shape", "lang", lang, "file", sub)
			continue
		}

		langs = append(langs, lang)

		_ = wfutils.Execute(ctx, activities.Vidispine.WaitForJobCompletion, vsactivity.WaitForJobCompletionParams{
			JobID:     jobRes.JobID,
			SleepTime: 10,
		}).Wait(ctx)
	}

	wfutils.SendTelegramText(ctx, telegram.ChatOther, "🟩 CSV based sub merge and import for VXID: "+params.TargetVXID+" finished")

	return true, nil
}

type SubtitleEntry struct {
	SubtransID  string `csv:"Subtrans ID"`
	TimecodeStr string `csv:"Timecode start"`
}

func parseSubMergeData(input []byte, separator rune) ([]SubtitleEntry, error) {
	var entries []SubtitleEntry

	gocsv.SetCSVReader(func(in io.Reader) gocsv.CSVReader {
		r := csv.NewReader(in)
		r.Comma = separator
		return r
	})

	if err := gocsv.UnmarshalBytes(input, &entries); err != nil {
		return nil, err
	}

	return entries, nil
}
