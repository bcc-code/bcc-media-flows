package vidispine

import (
	"fmt"
	"github.com/bcc-code/bcc-media-platform/backend/asset"
	"math"
	"regexp"
	"strings"

	"github.com/bcc-code/bccm-flows/services/vidispine/vsapi"
	"github.com/bcc-code/bccm-flows/services/vidispine/vscommon"
	"github.com/orsinium-labs/enum"
	"github.com/samber/lo"
)

type ChapterType enum.Member[string]

var (
	ChapterTypeSong      = ChapterType{"song"}
	ChapterTypeSpeech    = ChapterType{"speech"}
	ChapterTypeTestimony = ChapterType{"testimony"}
	ChapterTypeOther     = ChapterType{"other"}
	ChapterTypeCredits   = ChapterType{"credits"}

	ChapterTypeMap = map[string]ChapterType{
		"sang":           ChapterTypeSong,
		"musikkvideo":    ChapterTypeSong,
		"musikal":        ChapterTypeSong,
		"tale":           ChapterTypeSpeech,
		"appelle":        ChapterTypeSpeech,
		"vitnesbyrd":     ChapterTypeTestimony,
		"end-credit":     ChapterTypeCredits,
		"panel":          ChapterTypeOther,
		"intervju":       ChapterTypeOther,
		"temafilm":       ChapterTypeOther,
		"animasjon":      ChapterTypeOther,
		"programleder":   ChapterTypeOther,
		"dokumentar":     ChapterTypeOther,
		"ordforklaring":  ChapterTypeOther,
		"frsending":      ChapterTypeOther,
		"ettersending":   ChapterTypeOther,
		"bildekavalkade": ChapterTypeOther,
		"skuespill":      ChapterTypeOther,
		"aksjonstatus":   ChapterTypeOther,
		"hilse":          ChapterTypeOther,
		"konkuranse":     ChapterTypeOther,
		"informasjon":    ChapterTypeOther,
		"bnn":            ChapterTypeOther,
		"promo":          ChapterTypeOther,
		"mte":            ChapterTypeOther,
		"fest":           ChapterTypeOther,
		"underholdning":  ChapterTypeOther,
		"kortfilm":       ChapterTypeOther,
		"anslag":         ChapterTypeOther,
		"teaser":         ChapterTypeOther,
		"reality":        ChapterTypeOther,
		"studio":         ChapterTypeOther,
		"talk-show":      ChapterTypeOther,
		"presentasjon":   ChapterTypeOther,
		"seminar":        ChapterTypeOther,
		"reportasje":     ChapterTypeOther,
	}
)

var SongExtract = regexp.MustCompile("(FMB|HV) ?-? ?([0-9]+)")
var SongCollectionMap = map[string]string{
	"FMB": "AB",
	"HV":  "WOTL",
}

func (s *VidispineService) GetChapterData(exportData *ExportData) ([]asset.Chapter, error) {
	metaCache := map[string]*vsapi.MetadataResult{}

	allChapters := map[string]*vsapi.MetadataResult{}

	for _, clip := range exportData.Clips {
		if _, ok := metaCache[clip.VXID]; !ok {
			meta, err := s.apiClient.GetMetadata(clip.VXID)
			if err != nil {
				return nil, err
			}
			metaCache[clip.VXID] = meta
		}

		sourceMeta := metaCache[clip.VXID]
		startTC := sourceMeta.Get(vscommon.FieldStartTC, "0")
		tcStartSeconds, _ := vscommon.TCToSeconds(startTC)

		// The result here is in TC of the original MEDIA.
		chapterMeta, err := s.apiClient.GetChapterMeta(clip.VXID, clip.InSeconds+tcStartSeconds, clip.OutSeconds+tcStartSeconds)
		if err != nil {
			return nil, err
		}

		for title, data := range chapterMeta {
			// We need to convert the timestamps from Vidispine into something we can calculate with on sequence level
			data := convertFromClipTCTimeToSequenceRelativeTime(clip, data, tcStartSeconds)

			// We don't have this chapter yet
			if _, ok := allChapters[title]; !ok {
				allChapters[title] = data
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
					allChapters[title].Terse[name][i].Start = fmt.Sprintf("%.0f@PAL", newIn*25)
					allChapters[title].Terse[name][i].End = fmt.Sprintf("%.0f@PAL", newOut*25)
				}
			}
		}
	}

	var chapters []asset.Chapter
	for _, data := range allChapters {
		chapters = append(chapters, metaToChapter(data))
	}

	return chapters, nil
}

func metaToChapter(meta *vsapi.MetadataResult) asset.Chapter {
	out := asset.Chapter{}

	out.Label = meta.Get(vscommon.FieldTitle, "")
	out.Title = meta.Get(vscommon.FieldTitle, "")
	start, _ := vscommon.TCToSeconds(meta.Terse["title"][0].Start)
	out.Timestamp = start

	if chapterType, ok := ChapterTypeMap[meta.Get(vscommon.FieldSubclipType, "")]; ok {
		out.ChapterType = chapterType.Value
	} else {
		out.ChapterType = ChapterTypeOther.Value
	}

	// This is more or less useless
	// out.Description = meta.Get(FieldDescription, "")

	out.Highlight = false // When do we set this?

	out.Persons = lo.Filter(meta.GetArray(vscommon.FieldPersonsAppearing), func(p string, _ int) bool { return p != "" })

	if out.ChapterType == ChapterTypeSong.Value {
		match := SongExtract.FindStringSubmatch(strings.ToUpper(out.Label))
		if len(match) == 3 {
			out.SongCollection = match[1]
			out.SongCollection = match[2]
		}
	}

	return out
}
