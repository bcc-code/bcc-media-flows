package export

import (
	"encoding/json"
	"fmt"
	"github.com/bcc-code/bcc-media-flows/activities"
	"github.com/bcc-code/bcc-media-flows/activities/vidispine"
	"github.com/bcc-code/bcc-media-flows/paths"
	"github.com/bcc-code/bcc-media-flows/services/directus"
	"github.com/bcc-code/bcc-media-flows/services/notion"
	"github.com/bcc-code/bcc-media-flows/services/vidispine/vsapi"
	"github.com/bcc-code/bcc-media-flows/services/vidispine/vscommon"
	"github.com/bcc-code/bcc-media-flows/utils"
	wfutils "github.com/bcc-code/bcc-media-flows/utils/workflows"
	"github.com/davecgh/go-spew/spew"
	"github.com/gin-gonic/gin"
	"github.com/gocarina/gocsv"
	"github.com/samber/lo"
	"go.temporal.io/sdk/workflow"
	"os"
	"strconv"
	"strings"
)

type BulkExportShortsInput struct {
	CollectionVXID string `json:"collectionVXID"`
}

type ShortsCsvRow struct {
	Title           string   `csv:"Title" notion:"Title"`
	Language        string   `csv:"Language" notion:"Language"`
	Label           string   `csv:"Label" notion:"Label"`
	Publishing      string   `csv:"Publish" notion:"Publish"`
	EpisodeID       string   `csv:"Episode ID" notion:"Episode ID"`
	InHm            string   `csv:"In" notion:"In"`
	OutHm           string   `csv:"Out" notion:"Out"`
	Status          string   `csv:"Asset status" notion:"Asset status"`
	Source          string   `csv:"Source" notion:"Source"`
	Type            []string `csv:"Type" notion:"Type"`
	Purpose         string   `csv:"Purpose" notion:"Purpose"`
	Quality         string   `csv:"Quality" notion:"Quality"`
	EditorialStatus string   `csv:"Editorial status" notion:"Editorial status"`
}

func triggerShortExport(ctx workflow.Context, short *ShortsData) error {
	watermarkPath := ""

	exists, err := wfutils.Execute(ctx, activities.Directus.CheckDirectusAssetExists, short.MBMetadata.ID).Result(ctx)

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

// BulkExportShorts exports all shorts in a collection
//
// They must be in the CSV file, and have the editorial status of "Ready in MB" as well as a status of !"Done"
// The separate shorts are exported in parallel using the ExportShort workflow
func BulkExportShorts(ctx workflow.Context, input BulkExportShortsInput) error {
	if input.CollectionVXID == "" {
		return fmt.Errorf("collection VXID is required")
	}

	logger := workflow.GetLogger(ctx)
	logger.Info("Starting BulkExportShorts %s", input.CollectionVXID)

	shortsNotionDBID := "6656b0ef3b384b04993e144ed6e7feac"

	notionFilter, _ := json.Marshal(gin.H{
		"and": []gin.H{
			{
				"property": "Editorial status",
				"status": gin.H{
					"equals": "Ready in MB",
				},
			}, {
				"property": "Asset status",
				"status": gin.H{
					"does_not_equal": "Done",
				},
			},
		},
	})

	spew.Dump(string(notionFilter))

	rawNotionData, err := wfutils.Execute(ctx, activities.Notion.QueryDatabase, activities.QueryDatabaseArgs{
		DatabaseID: shortsNotionDBID,
		Filter:     string(notionFilter),
	}).Result(ctx)

	if err != nil {
		return fmt.Errorf("failed to get notion data: %w", err)
	}

	data, err := notion.NotionToStruct[ShortsCsvRow](rawNotionData)
	if err != nil {
		return fmt.Errorf("failed to parse notion data: %w", err)
	}

	items, err := wfutils.Execute(ctx, activities.Vidispine.GetItemsInCollection, vsactivity.GetItemsInCollectionParams{
		CollectionID: input.CollectionVXID,
		Limit:        1000,
	}).Result(ctx)
	if err != nil {
		return fmt.Errorf("failed to get items from collection: %w", err)
	}

	dataPtrs := lo.Map(data, func(row ShortsCsvRow, _ int) *ShortsCsvRow { return &row })

	shortsData := MapAndFilterShortsData(dataPtrs, items)

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

// ExportShort exports a single short to BCCM Platform
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

	return createShortInPlatform(ctx, short, styledImage)
}

func createShortInPlatform(ctx workflow.Context, short *ShortsData, styledImage *directus.StyledImage) error {

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
	assetResult, err := wfutils.Execute(ctx, activities.Directus.GetAssetByMediabankenID, short.MBMetadata.ID).Result(ctx)
	if err != nil {
		return fmt.Errorf("failed to get asset: %w", err)
	}

	if assetResult == nil {
		return fmt.Errorf("no asset found for mediabanken_id: %s", short.MBMetadata.ID)
	}

	assetID := strconv.Itoa(int(assetResult.ID))

	var episodeID string
	if short.CSV.EpisodeID == "" {
		episodeID = short.CSV.EpisodeID
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

	if mediaItemResult == nil {
		return fmt.Errorf("failed to create media item: no data in response")
	}

	// some of the columns should be added as tags, e.g. "edification"
	tagCodes := []string{
		short.CSV.Source,
		short.CSV.Purpose,
	}
	tagCodes = append(tagCodes, short.CSV.Type...)

	for _, raw := range tagCodes {
		code := strings.ToLower(strings.TrimSpace(raw))
		if code == "" {
			continue
		}

		tagId, err := GetOrCreateTag(ctx, code)
		if err != nil {
			return err
		}
		spew.Dump(tagId)

		// Create relationship between media item and tag
		_, err = wfutils.Execute(ctx, activities.Directus.CreateMediaItemTag, activities.CreateMediaItemTagInput{
			MediaItemID: mediaItemResult.ID,
			TagID:       tagId,
		}).Result(ctx)
		if err != nil {
			workflow.GetLogger(ctx).Error("Error creating media item tag relationship",
				"error", err,
				"mediaItemID", mediaItemResult.ID,
				"tagId", tagId,
			)
		}
	}

	// Create short
	shortResult, err := wfutils.Execute(ctx, activities.Directus.CreateShort, activities.CreateShortInput{
		MediaItemID: mediaItemResult.ID,
		Status:      "draft",
	}).Result(ctx)

	if err != nil {
		return fmt.Errorf("failed to create short: %w", err)
	}

	if shortResult == nil {
		return fmt.Errorf("failed to create short: no data in response")
	}

	return nil
}

// GetOrCreateTag checks if a tag exists with the given code, creates it if it doesn't exist, and returns its ID
func GetOrCreateTag(ctx workflow.Context, code string) (string, error) {
	res, err := wfutils.Execute(ctx, activities.Directus.GetOrCreateTag, activities.GetOrCreateTagInput{
		Code: code,
		Name: code,
	}).Result(ctx)

	if err != nil {
		return "", err
	}

	return res.ID, nil
}

func getInOutTime(short *ShortsData) (*int64, *int64, error) {
	var parentStartsAt *int64
	var parentEndsAt *int64

	if strings.TrimSpace(short.CSV.InHm) != "" {
		inNum, err := convertToSeconds(short.CSV.InHm)
		parentStartsAt = inNum
		if err != nil {
			return nil, nil, err
		}
	}

	if strings.TrimSpace(short.CSV.OutHm) != "" {
		n, err := convertToSeconds(short.CSV.OutHm)
		parentEndsAt = n
		if err != nil {
			return nil, nil, err
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

	res, err := wfutils.Execute(ctx, activities.Directus.UploadFile, image.Local()).Result(ctx)

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

// ParseShortsCsvRows parses ShortsCsvRow from CSV data
func ParseShortsCsvRows(csvData []byte) ([]*ShortsCsvRow, error) {
	csvRows := []*ShortsCsvRow{}
	if err := gocsv.UnmarshalBytes(csvData, &csvRows); err != nil {
		return nil, err
	}
	return csvRows, nil
}
