package vidispine

import (
	"fmt"
	"math"
	"regexp"
	"strings"

	"github.com/bcc-code/bcc-media-platform/backend/asset"
	pcommon "github.com/bcc-code/bcc-media-platform/backend/common"

	"github.com/bcc-code/bcc-media-flows/services/vidispine/vsapi"
	"github.com/bcc-code/bcc-media-flows/services/vidispine/vscommon"
	"github.com/samber/lo"
)

var (
	ChapterTypeMap = map[string]pcommon.ChapterType{
		"sang":           pcommon.ChapterTypeSong,
		"musikkvideo":    pcommon.ChapterTypeSong,
		"musikal":        pcommon.ChapterTypeSong,
		"tale":           pcommon.ChapterTypeSpeech,
		"appelle":        pcommon.ChapterTypeSpeech,
		"vitnesbyrd":     pcommon.ChapterTypeTestimony,
		"end-credit":     pcommon.ChapterTypeOther,
		"singalong":      pcommon.ChapterTypeSingAlong,
		"panel":          pcommon.ChapterTypeOther,
		"intervju":       pcommon.ChapterTypeOther,
		"temafilm":       pcommon.ChapterTypeOther,
		"animasjon":      pcommon.ChapterTypeOther,
		"programleder":   pcommon.ChapterTypeOther,
		"dokumentar":     pcommon.ChapterTypeOther,
		"ordforklaring":  pcommon.ChapterTypeOther,
		"frsending":      pcommon.ChapterTypeOther,
		"ettersending":   pcommon.ChapterTypeOther,
		"bildekavalkade": pcommon.ChapterTypeOther,
		"skuespill":      pcommon.ChapterTypeOther,
		"aksjonstatus":   pcommon.ChapterTypeOther,
		"hilse":          pcommon.ChapterTypeOther,
		"konkuranse":     pcommon.ChapterTypeOther,
		"informasjon":    pcommon.ChapterTypeOther,
		"bnn":            pcommon.ChapterTypeOther,
		"promo":          pcommon.ChapterTypeOther,
		"mte":            pcommon.ChapterTypeOther,
		"fest":           pcommon.ChapterTypeOther,
		"underholdning":  pcommon.ChapterTypeOther,
		"kortfilm":       pcommon.ChapterTypeOther,
		"anslag":         pcommon.ChapterTypeOther,
		"teaser":         pcommon.ChapterTypeOther,
		"reality":        pcommon.ChapterTypeOther,
		"studio":         pcommon.ChapterTypeOther,
		"talk-show":      pcommon.ChapterTypeOther,
		"presentasjon":   pcommon.ChapterTypeOther,
		"seminar":        pcommon.ChapterTypeOther,
		"reportasje":     pcommon.ChapterTypeOther,
		"tydning":        pcommon.ChapterTypeOther,
	}
)

var typesToFilterOut = []string{
	"end-credit",
	"tydning",
	"bnn",
	"frsending",
	"ettersending",
	"programleder",
}

var prioritizedTypeOrder = []pcommon.ChapterType{
	pcommon.ChapterTypeInterview,
	pcommon.ChapterTypeTheme,
	pcommon.ChapterTypeSpeech,
	pcommon.ChapterTypeSingAlong,
	pcommon.ChapterTypeSong,
}

func mapSubclipType(vsChapterType string) pcommon.ChapterType {
	if chapterType, ok := ChapterTypeMap[vsChapterType]; ok {
		return chapterType
	}
	return pcommon.ChapterTypeOther
}

var SongExtract = regexp.MustCompile("(FMB|HV) ?-? ?([0-9]+)")
var SongCollectionMap = map[string]string{
	"FMB": "AB",
	"HV":  "WOTL",
}

func GetChapterData(client Client, exportData *ExportData) ([]asset.TimedMetadata, error) {
	metaCache := map[string]*vsapi.MetadataResult{}

	allChapters := map[string]*vsapi.MetadataResult{}

	originalStart := map[string]float64{}

	for _, clip := range exportData.Clips {
		if _, ok := metaCache[clip.VXID]; !ok {
			meta, err := client.GetMetadata(clip.VXID)
			if err != nil {
				return nil, err
			}
			metaCache[clip.VXID] = meta
		}

		sourceMeta := metaCache[clip.VXID]
		startTC := sourceMeta.Get(vscommon.FieldStartTC, "0")
		tcStartSeconds, _ := vscommon.TCToSeconds(startTC)

		// The result here is in TC of the original MEDIA.
		chapterMeta, err := client.GetChapterMeta(clip.VXID, clip.InSeconds+tcStartSeconds, clip.OutSeconds+tcStartSeconds)
		if err != nil {
			return nil, err
		}

		for title, data := range chapterMeta {
			// We need to convert the timestamps from Vidispine into something we can calculate with on sequence level
			data := convertFromClipTCTimeToSequenceRelativeTime(clip, data, tcStartSeconds)

			// We don't have this chapter yet
			if _, ok := allChapters[title]; !ok {
				allChapters[title] = data
				originalStart[title] = clip.InSeconds
				continue
			}

			// This chapter already exists, so we need to merge the data.
			// Since the source is the same the only diff is the in and out point
			// i.e. we only need the earlies in and latest out point on all values

			tcIn1, _ := vscommon.TCToSeconds(data.Terse["title"][0].Start)
			tcOut1, _ := vscommon.TCToSeconds(data.Terse["title"][0].End)

			tcIn2, _ := vscommon.TCToSeconds(allChapters[title].Terse["title"][0].Start)
			tcOut2, _ := vscommon.TCToSeconds(allChapters[title].Terse["title"][0].End)

			newIn := math.Min(tcIn1, tcIn2)
			newOut := math.Max(tcOut1, tcOut2)

			for name := range allChapters[title].Terse {
				for i := range allChapters[title].Terse[name] {
					originalStart[title] = tcIn1
					allChapters[title].Terse[name][i].Start = fmt.Sprintf("%.0f@PAL", newIn*25)
					allChapters[title].Terse[name][i].End = fmt.Sprintf("%.0f@PAL", newOut*25)
				}
			}
		}
	}

	var chapters []asset.TimedMetadata
	for _, data := range allChapters {
		chapter, keep := metaToChapter(data)
		if !keep {
			continue
		}
		if chapter.Timestamp == 0 {
			chapter.Timestamp = originalStart[data.Get(vscommon.FieldTitle, "")]
		}
		chapters = append(chapters, chapter)
	}

	return chapters, nil
}

func metaToChapter(meta *vsapi.MetadataResult) (asset.TimedMetadata, bool) {
	out := asset.TimedMetadata{}

	out.Label = meta.Get(vscommon.FieldTitle, "")
	out.Title = meta.Get(vscommon.FieldTitle, "")
	start, _ := vscommon.TCToSeconds(meta.Terse["title"][0].Start)
	out.Timestamp = start

	subclipTypes := meta.GetArray(vscommon.FieldSubclipType)
	if len(subclipTypes) == 0 {
		return out, false
	}
	if lo.Contains(typesToFilterOut, subclipTypes[0]) {
		return out, false
	}
	subclipType, chapterType := findBestChapterType(subclipTypes)
	out.ChapterType = chapterType.Value

	out.Persons = lo.Filter(meta.GetArray(vscommon.FieldPersonsAppearing), func(p string, _ int) bool { return p != "" })

	if out.ChapterType == pcommon.ChapterTypeSong.Value || out.ChapterType == pcommon.ChapterTypeSingAlong.Value {
		match := SongExtract.FindStringSubmatch(strings.ToUpper(out.Label))
		if len(match) == 3 {
			out.SongCollection = match[1]
			out.SongNumber = match[2]
		}
	}

	if out.ChapterType == pcommon.ChapterTypeOther.Value {
		out.Title = subclipType
	}
	if strings.Contains(out.Label, " - ") {
		out.Title = strings.Split(out.Label, " - ")[0]
	}

	return out, true
}

func findBestChapterType(subclipTypes []string) (string, *pcommon.ChapterType) {
	if len(subclipTypes) > 1 {
		for _, t := range subclipTypes {
			c := mapSubclipType(t)
			if c == pcommon.ChapterTypeOther {
				return t, &c
			}
		}
		for _, t := range prioritizedTypeOrder {
			for _, subclipType := range subclipTypes {
				t2 := mapSubclipType(subclipType)
				if t.Value == t2.Value {
					return subclipType, &t
				}
			}
		}
	}

	chapterType := mapSubclipType(subclipTypes[0])
	return subclipTypes[0], &chapterType
}
