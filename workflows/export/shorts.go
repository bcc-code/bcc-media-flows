package export

import (
	"fmt"
	"github.com/bcc-code/bcc-media-flows/activities"
	"github.com/bcc-code/bcc-media-flows/activities/vidispine"
	"github.com/bcc-code/bcc-media-flows/directus"
	"github.com/bcc-code/bcc-media-flows/paths"
	"github.com/bcc-code/bcc-media-flows/services/vidispine/vsapi"
	"github.com/bcc-code/bcc-media-flows/services/vidispine/vscommon"
	"github.com/bcc-code/bcc-media-flows/utils"
	wfutils "github.com/bcc-code/bcc-media-flows/utils/workflows"
	"github.com/gocarina/gocsv"
	"go.temporal.io/sdk/workflow"
	"os"
	"strconv"
	"strings"
)

type BulkExportShortsInput struct {
	CSV            string `json:"csv"`
	CollectionVXID string `json:"collectionVXID"`
}

type DirectusAPI interface {
	AssetExists(id string) (bool, error)
	CreateMediaItemStyledImage(mediaItemID, styledImageID string) error
}

func triggerShortExport(ctx workflow.Context, short *ShortsData) error {
	watermarkPath := ""

	exists, err := wfutils.Execute(ctx, activities.Directus.CheckDirectusAssetExists, activities.CheckDirectusAssetExistsInput{
		MediabankenID: short.MBMetadata.ID,
	}).Result(ctx)

	if err != nil {
		return err
	}

	if exists {
		return nil
	}

	resolutions := []utils.Resolution{
		{Width: 320, Height: 180},
		{Width: 480, Height: 270},
		{Width: 640, Height: 360},
		{Width: 960, Height: 540},
		{Width: 1280, Height: 720},
		{Width: 1920, Height: 1080},
	}

	wf := workflow.ExecuteChildWorkflow(ctx, VXExport, VXExportParams{
		VXID:          short.MBMetadata.ID,
		Destinations:  []string{"vod"},
		WatermarkPath: watermarkPath,
		Resolutions:   resolutions,
		AudioSource:   "embedded",
	})

	return wf.Get(ctx, nil)
}

type ShortsData struct {
	CSV          *ShortsCsvRow
	MBMetadata   *vsapi.MetadataResult
	OriginalPath *paths.Path
}

// MapAndFilterShortsData matches csv rows with vx items and returns a filtered list of matched items.
// Only editorial status "Ready in MB" is allowed
func MapAndFilterShortsData(csvRows []*ShortsCsvRow, mbItems []*vsapi.MetadataResult) []*ShortsData {
	var out []*ShortsData

	mbItemMap := make(map[string]*vsapi.MetadataResult, len(mbItems))
	for _, item := range mbItems {
		title := item.Get(vscommon.FieldTitle, "")
		if title == "" {
			continue
		}

		baseTitle := title
		if dotIdx := strings.IndexByte(title, '.'); dotIdx > 0 {
			baseTitle = title[:dotIdx]
		}

		mbItemMap[baseTitle] = item
	}

	for _, csvRow := range csvRows {
		if csvRow.EditorialStatus != "Ready in MB" {
			continue
		}

		if csvRow.Status == "Done" {
			continue
		}

		if item, ok := mbItemMap[csvRow.Label]; ok {

			if csvRow.Language == "" {
				csvRow.Language = "nor"
			}

			out = append(out, &ShortsData{
				CSV:        csvRow,
				MBMetadata: item,
			})
		}
	}

	return out
}

func BulkExportShorts(ctx workflow.Context, input BulkExportShortsInput) error {
	if input.CollectionVXID == "" {
		return fmt.Errorf("collection VXID is required")
	}

	if input.CSV == "" {
		return fmt.Errorf("CSV data is required")
	}

	logger := workflow.GetLogger(ctx)
	logger.Info("Starting BulkExportShorts %s", input.CollectionVXID)

	items, err := wfutils.Execute(ctx, activities.Vidispine.GetItemsInCollection, vsactivity.GetItemsInCollectionParams{
		CollectionID: input.CollectionVXID,
		Limit:        1000,
	}).Result(ctx)
	if err != nil {
		return fmt.Errorf("failed to get items from collection: %w", err)
	}

	csvRows, err := ParseShortsCsvRows([]byte(input.CSV))
	if err != nil {
		return err
	}

	shortsData := MapAndFilterShortsData(csvRows, items)

	wfs := make([]workflow.Future, len(shortsData))
	for i, short := range shortsData {
		wf := workflow.ExecuteChildWorkflow(ctx, ExportShort, short)
		wfs[i] = wf
	}

	errors := []error{}
	for _, wf := range wfs {
		err = wf.Get(ctx, nil)
		if err != nil {
			errors = append(errors, err)
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("errors: %v", errors)
	}

	return nil
}

func ExportShort(ctx workflow.Context, short *ShortsData) error {
	logger := workflow.GetLogger(ctx)
	logger.Debug("Starting export for %s", short.MBMetadata.ID)

	tempFolder, err := wfutils.GetWorkflowTempFolder(ctx)
	if err != nil {
		return fmt.Errorf("failed to get temp folder: %w", err)
	}

	err = triggerShortExport(ctx, short)
	if err != nil {
		return fmt.Errorf("failed to trigger export: %w", err)
	}

	res, err := wfutils.Execute(ctx, activities.Vidispine.GetFileFromVXActivity, vsactivity.GetFileFromVXParams{
		VXID: short.MBMetadata.ID,
		Tags: []string{"original"},
	}).Result(ctx)
	if err != nil {
		return err
	}

	short.OriginalPath = &res.FilePath

	thumb, err := generateThumbnailForShort(ctx, tempFolder, short)
	if err != nil {
		return fmt.Errorf("failed to generate thumbnail: %w", err)
	}

	_, styledImage, err := uploadImage(ctx, true, "poster", thumb)
	if err != nil {
		return fmt.Errorf("failed to upload thumbnail: %w", err)
	}

	return importShort(ctx, short, styledImage)
}

func importShort(ctx workflow.Context, short *ShortsData, styledImage *directus.StyledImage) error {

	// Create mediaitem
	language := short.CSV.Language
	if language == "" {
		language = "no"
	}

	parentStartsAt, parentEndsAt, err := getInOutTime(short)
	if err != nil {
		return err
	}

	// Get asset for item
	assetResult, err := wfutils.Execute(ctx, activities.Directus.GetAssetByMediabankenID, activities.GetAssetByMediabankenIDInput{
		MediabankenID: short.MBMetadata.ID,
	}).Result(ctx)
	if err != nil {
		return fmt.Errorf("failed to get asset: %w", err)
	}

	if assetResult.Asset == nil {
		return fmt.Errorf("no asset found for mediabanken_id: %s", short.MBMetadata.ID)
	}

	assetID := assetResult.Asset.ID

	var episodeID *int
	if short.CSV.EpisodeID != 0 {
		episodeID = &short.CSV.EpisodeID
	} else {
		fmt.Printf("WARN: EpisodeID is empty for %s, %s\n", short.MBMetadata.ID, short.CSV.Label)
	}

	label := short.CSV.Label
	if label == "" {
		label = short.MBMetadata.Get(vscommon.FieldTitle, "<NO TITLE>")
	}

	// Create media item
	mediaItemResult, err := wfutils.Execute(ctx, activities.Directus.CreateMediaItem, activities.CreateMediaItemInput{
		Label:           label,
		Type:            "short",
		AssetID:         assetID,
		Title:           "",
		ParentEpisodeID: episodeID,
		ParentStartsAt:  parentStartsAt,
		ParentEndsAt:    parentEndsAt,
		StyledImageID:   styledImage.ID,
	}).Result(ctx)

	if err != nil {
		return fmt.Errorf("failed to create media item: %w", err)
	}

	if mediaItemResult.MediaItem == nil {
		return fmt.Errorf("failed to create media item: no data in response")
	}

	/*
		TODO: Add tags
		// some of the columns should be added as tags, e.g. "edification"
		tagCodes := []string{
			short.CSV.Source,
			short.CSV.Type,
			short.CSV.Purpose,
		}

		for _, raw := range tagCodes {
			code := strings.ToLower(strings.TrimSpace(raw))
			if code == "" {
				continue
			}
			tagId, err := utils.GetOrCreateTag(db, code)
			if err != nil {
				return err
			}
			// insert into mediaitems_tags id, mediaitems_id, tags_id
			query := "INSERT INTO mediaitems_tags (mediaitems_id, tags_id) VALUES ($1, $2)"
			_, err = db.Exec(query, mediaItemID, tagId)
			if err != nil {
				fmt.Printf("Error: inserting into mediaitems_tags: %s\n", err)
			}
		}
	*/

	// Create short
	shortResult, err := wfutils.Execute(ctx, activities.Directus.CreateShort, activities.CreateShortInput{
		MediaItemID: mediaItemResult.MediaItem.ID,
		Status:      "draft",
	}).Result(ctx)

	if err != nil {
		return fmt.Errorf("failed to create short: %w", err)
	}

	if shortResult.Short == nil {
		return fmt.Errorf("failed to create short: no data in response")
	}

	return nil
}

func getInOutTime(short *ShortsData) (*int64, *int64, error) {
	var parentStartsAt *int64
	var parentEndsAt *int64

	if strings.TrimSpace(short.CSV.InNum) != "" {
		inNum, err := strconv.ParseInt(short.CSV.InNum, 10, 64)
		parentStartsAt = &inNum
		if err != nil {
			return nil, nil, err
		}
	} else {
		if strings.TrimSpace(short.CSV.InHm) != "" {
			inNum, err := convertToSeconds(short.CSV.InHm)
			parentStartsAt = inNum
			if err != nil {
				return nil, nil, err
			}
		}
	}

	if strings.TrimSpace(short.CSV.OutNum) != "" {
		outNum, err := strconv.ParseInt(short.CSV.OutNum, 10, 64)
		parentEndsAt = &outNum
		if err != nil {
			return nil, nil, err
		}
	} else {
		if strings.TrimSpace(short.CSV.OutHm) != "" {
			n, err := convertToSeconds(short.CSV.OutHm)
			parentEndsAt = n
			if err != nil {
				return nil, nil, err
			}
		}
	}

	if parentStartsAt != nil && parentEndsAt != nil && *parentStartsAt > *parentEndsAt {
		fmt.Printf("ERROR: csvRow InNum is greater than OutNum. Skipping %s, %s\n", short.MBMetadata.ID, short.CSV.Label)
		return nil, nil, nil
	}
	if parentStartsAt == nil || parentEndsAt == nil {
		parentStartsAt = new(int64)
		*parentStartsAt = int64(0)
		parentEndsAt = new(int64)
		*parentEndsAt = int64(0)
		fmt.Printf("WARNING: In/Out was not found for %s, %s\n", short.MBMetadata.ID, short.CSV.Label)
		//return nil
	}
	return parentStartsAt, parentEndsAt, nil
}

// convertToSeconds takes a string in the format "HH:MM:SS" and converts it to seconds.
// HH is optional
func convertToSeconds(timeStr string) (*int64, error) {
	parts := strings.Split(timeStr, ":")
	if len(parts) == 2 {
		parts = append([]string{"0"}, parts...)
	}

	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid time format: %s", timeStr)
	}

	hours, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return nil, err
	}

	minutes, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return nil, err
	}

	seconds, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil {
		return nil, err
	}

	totalSeconds := hours*60*60 + minutes*60 + seconds

	return &totalSeconds, nil
}

func uploadImage(ctx workflow.Context, createStyledImages bool, imageStyle string, image paths.Path) (*directus.File, *directus.StyledImage, error) {
	if !strings.HasSuffix(image.Ext(), ".jpg") {
		return nil, nil, fmt.Errorf("invalid image extension: %s", image.Ext())
	}

	res, err := wfutils.Execute(ctx, activities.Directus.UploadFile, activities.UploadFileInput{
		FilePath: image.Local(),
	}).Result(ctx)

	if err != nil {
		return nil, nil, err
	}

	if createStyledImages && imageStyle != "" {
		styledImage, err := wfutils.Execute(ctx, activities.Directus.CreateStyledImage, activities.CreateStyledImageInput{
			ImageID: res.ID,
			Style:   imageStyle,
		}).Result(ctx)

		return res, styledImage, err
	}

	return res, nil, err
}

func generateThumbnailForShort(ctx workflow.Context, destFolder paths.Path, short *ShortsData) (paths.Path, error) {

	filePath := short.OriginalPath

	outputFilePath := destFolder.Append(short.MBMetadata.ID + ".jpg")

	_, err := os.Stat(outputFilePath.Local())
	if err == nil {
		return outputFilePath, nil
	}

	ffmpegArgs := []string{"-i", filePath.Local(), "-vf", "select=eq(n\\,29)", "-vframes", "1", "-q:v", "3", "-update", "1", outputFilePath.Local()}

	err = wfutils.Execute(ctx, activities.Video.ExecuteFFmpeg, activities.ExecuteFFmpegInput{Arguments: ffmpegArgs}).Wait(ctx)
	return outputFilePath, err
}

type ShortsCsvRow struct {
	Title             string `csv:"Title"`
	Language          string `csv:"Language"`
	Label             string `csv:"Label"`
	EditorialApproved string `csv:"Editorial approved"`
	Publishing        string `csv:"Publishing"`
	EpisodeID         int    `csv:"Episode ID"`
	InHm              string `csv:"In"`
	OutHm             string `csv:"Out"`
	InNum             string `csv:"In Sec"`
	OutNum            string `csv:"Out Sec"`
	LanguageCheck     string `csv:"Language check"`
	Comments          string `csv:"Comments"`
	Platform          string `csv:"Platform"`
	Status            string `csv:"Asset status"`
	Source            string `csv:"Source"`
	Type              string `csv:"Type"`
	Purpose           string `csv:"Purpose"`
	Quality           string `csv:"Quality"`
	EditorialStatus   string `csv:"Editorial status"`
}

// ShortLanguageUpdate holds info for later processing
type ShortLanguageUpdate struct {
	VXID     string
	Title    string
	Language string
}

// ParseShortsCsvRows parses ShortsCsvRow from CSV data
func ParseShortsCsvRows(csvData []byte) ([]*ShortsCsvRow, error) {
	csvRows := []*ShortsCsvRow{}
	if err := gocsv.UnmarshalBytes(csvData, &csvRows); err != nil {
		return nil, err
	}
	return csvRows, nil
}
