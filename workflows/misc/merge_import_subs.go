package miscworkflows

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"strings"
	"time"

	"github.com/bcc-code/bcc-media-flows/activities"
	vsactivity "github.com/bcc-code/bcc-media-flows/activities/vidispine"
	"github.com/bcc-code/bcc-media-flows/common"
	"github.com/bcc-code/bcc-media-flows/paths"
	"github.com/bcc-code/bcc-media-flows/services/telegram"
	wfutils "github.com/bcc-code/bcc-media-flows/utils/workflows"
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

		for _, lang := range wfutils.SortedKeys(res) {
			sub := res[lang]

			if _, ok := mergeData[lang]; !ok {
				mergeData[lang] = &common.MergeInput{
					Title:     params.Title + "_" + lang,
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

		jobRes, err := wfutils.Execute(ctx, activities.Vidispine.ImportFileAsShapeActivity, vsactivity.ImportFileAsShapeParams{
			AssetID:  params.TargetVXID,
			FilePath: sub,
			ShapeTag: fmt.Sprintf("sub_%s_%s", lang, "srt"),
			Replace:  true,
		}).Result(ctx)

		if err != nil {
			return false, err
		}

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
	SubtransID  string
	TimecodeStr string
}

const (
	subtransIDColumn = "Subtrans ID"
	timecodeColumn   = "Timecode start"
)

// parseSubMergeData reads the two columns this workflow needs out of the
// uploaded CSV, matching them by header name so extra columns and column
// reordering are both tolerated.
func parseSubMergeData(input []byte, separator rune) ([]SubtitleEntry, error) {
	reader := csv.NewReader(bytes.NewReader(input))
	reader.Comma = separator

	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, nil
	}

	idIndex, timecodeIndex := -1, -1
	for i, name := range records[0] {
		switch strings.TrimSpace(name) {
		case subtransIDColumn:
			idIndex = i
		case timecodeColumn:
			timecodeIndex = i
		}
	}
	if idIndex < 0 || timecodeIndex < 0 {
		return nil, fmt.Errorf("CSV is missing a %q or %q column, got: %s",
			subtransIDColumn, timecodeColumn, strings.Join(records[0], string(separator)))
	}

	entries := make([]SubtitleEntry, 0, len(records)-1)
	for _, record := range records[1:] {
		entries = append(entries, SubtitleEntry{
			SubtransID:  record[idIndex],
			TimecodeStr: record[timecodeIndex],
		})
	}

	return entries, nil
}
