package platform_activities

import (
	"context"
	"regexp"
	"strings"

	cantemoactivities "github.com/bcc-code/bcc-media-flows/activities/cantemo"
	vsactivity "github.com/bcc-code/bcc-media-flows/activities/vidispine"
	"github.com/bcc-code/bcc-media-flows/services/cantemo"
	"github.com/bcc-code/bcc-media-flows/services/vidispine"
	"github.com/bcc-code/bcc-media-flows/services/vidispine/vsapi"
	"github.com/bcc-code/bcc-media-flows/services/vidispine/vscommon"
	"github.com/bcc-code/bcc-media-platform/backend/asset"
	pcommon "github.com/bcc-code/bcc-media-platform/backend/common"
	"github.com/samber/lo"
	"go.temporal.io/sdk/activity"
)

type Activities struct{}

var PlatformActivities = Activities{}

type GetTimedMetadataChaptersParams struct {
	Clips []*vidispine.Clip
}

func (a Activities) GetTimedMetadataChaptersActivity(ctx context.Context, params GetTimedMetadataChaptersParams) ([]asset.TimedMetadata, error) {
	log := activity.GetLogger(ctx)
	activity.RecordHeartbeat(ctx, "GetTimedMetadataChaptersActivity")
	log.Info("Starting GetTimedMetadataChaptersActivity")

	vsClient := vsactivity.GetClient()
	cantemoClient := cantemoactivities.GetClient()

	return GetTimedMetadataChapters(vsClient, cantemoClient, params.Clips)
}

func GetTimedMetadataChapters(vsClient vidispine.Client, cantemoClient *cantemo.Client, clips []*vidispine.Clip) ([]asset.TimedMetadata, error) {
	vsChapters, err := vidispine.GetChapterMetaForClips(vsClient, clips)
	if err != nil {
		return nil, err
	}

	subclipTypeNames, err := cantemoClient.GetLookupChoices("Subclips", vscommon.FieldSubclipType.Value)
	if err != nil {
		return nil, err
	}

	var chapters []asset.TimedMetadata
	for _, data := range vsChapters {
		chapter, keep := metaToChapter(data.Meta, subclipTypeNames)
		if !keep {
			continue
		}
		if chapter.Timestamp == 0 {
			chapter.Timestamp = data.OriginalStart
		}
		chapters = append(chapters, chapter)
	}

	return chapters, nil
}

func metaToChapter(meta *vsapi.MetadataResult, subclipTypeNames map[string]string) (asset.TimedMetadata, bool) {
	out := asset.TimedMetadata{}

	start, _ := vscommon.TCToSeconds(meta.Terse["title"][0].Start)
	out.Timestamp = start

	subclipTypes := meta.GetArray(vscommon.FieldSubclipType)
	if len(subclipTypes) == 0 {
		return out, false
	}
	if lo.Contains(chapterTypesToFilterOut, subclipTypes[0]) {
		return out, false
	}
	subclipType, chapterType := findBestContentType(subclipTypes)
	out.ContentType = chapterType.Value

	out.Persons = lo.Filter(meta.GetArray(vscommon.FieldPersonsAppearing), func(p string, _ int) bool { return p != "" })

	out.Label = meta.Get(vscommon.FieldTitle, "")
	out.Title = meta.Get(vscommon.FieldTitle, "")

	if out.ContentType == pcommon.ContentTypeSong.Value || out.ContentType == pcommon.ContentTypeSingAlong.Value {
		match := SongExtract.FindStringSubmatch(strings.ToUpper(out.Label))
		if len(match) == 3 {
			out.SongCollection = match[1]
			out.SongNumber = match[2]
		}
	}

	if meta.Get(vscommon.FieldSubclipExportTitle, "") != "yes" {
		if out.ContentType == pcommon.ContentTypeOther.Value {
			if typeName, ok := subclipTypeNames[subclipType]; ok {
				out.Title = typeName
			} else {
				out.Title = subclipType
			}
		} else if strings.Contains(out.Title, " - ") {
			out.Title = strings.Split(out.Title, " - ")[0]
		}
	}

	return out, true
}

var SongExtract = regexp.MustCompile("(FMB|HV) ?-? ?([0-9]+)")
var SongCollectionMap = map[string]string{
	"FMB": "AB",
	"HV":  "WOTL",
}
var (
	chapterTypeMap = map[string]pcommon.ContentType{
		"sang":        pcommon.ContentTypeSong,
		"musikkvideo": pcommon.ContentTypeSong,
		"singalong":   pcommon.ContentTypeSingAlong,
		"tale":        pcommon.ContentTypeSpeech,
		"vitnesbyrd":  pcommon.ContentTypeSpeech,
		"appelle":     pcommon.ContentTypeSpeech,
		"panel":       pcommon.ContentTypeInterview,
		"intervju":    pcommon.ContentTypeInterview,
		"kortfilm":    pcommon.ContentTypeTheme,
		"anslag":      pcommon.ContentTypeTheme,
		"temafilm":    pcommon.ContentTypeTheme,
	}
)

var chapterTypesToFilterOut = []string{
	"end-credit",
	"tydning",
	"bnn",
	"frsending",
	"ettersending",
	"programleder",
}

func mapSubclipType(vsContentType string) pcommon.ContentType {
	if chapterType, ok := chapterTypeMap[vsContentType]; ok {
		return chapterType
	}
	return pcommon.ContentTypeOther
}

func findBestContentType(subclipTypes []string) (string, *pcommon.ContentType) {
	if len(subclipTypes) > 1 {
		for _, prioritizedType := range []pcommon.ContentType{
			pcommon.ContentTypeOther,
			pcommon.ContentTypeInterview,
			pcommon.ContentTypeTheme,
			pcommon.ContentTypeSpeech,
			pcommon.ContentTypeSingAlong,
			pcommon.ContentTypeSong,
		} {
			for _, subclipType := range subclipTypes {
				chapterType := mapSubclipType(subclipType)
				if chapterType.Value == prioritizedType.Value {
					return subclipType, &prioritizedType
				}
			}
		}
	}

	chapterType := mapSubclipType(subclipTypes[0])
	return subclipTypes[0], &chapterType
}
