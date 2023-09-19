package vidispine

import (
	"fmt"
	"math"
	"regexp"
	"strings"

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
		"sang":            ChapterTypeSong,
		"musikkvideo":     ChapterTypeSong,
		"musikal":         ChapterTypeSong,
		"tale":            ChapterTypeSpeech,
		"appelle":         ChapterTypeSpeech,
		"vitnesbyrd":      ChapterTypeTestimony,
		"end-credit":      ChapterTypeCredits,
		"panel":           ChapterTypeOther,
		"intervju":        ChapterTypeOther,
		"temafilm":        ChapterTypeOther,
		"animation":       ChapterTypeOther,
		"programleder":    ChapterTypeOther,
		"dokumentar":      ChapterTypeOther,
		"ordforklaring":   ChapterTypeOther,
		"frsending":       ChapterTypeOther,
		"ettersending":    ChapterTypeOther,
		"bildekavaalkade": ChapterTypeOther,
		"skuespill":       ChapterTypeOther,
		"aksjonstatus":    ChapterTypeOther,
		"hilse":           ChapterTypeOther,
		"konkuranse":      ChapterTypeOther,
		"informasjon":     ChapterTypeOther,
		"bnn":             ChapterTypeOther,
		"promo":           ChapterTypeOther,
		"mte":             ChapterTypeOther,
		"fest":            ChapterTypeOther,
		"underholdning":   ChapterTypeOther,
		"kortfilm":        ChapterTypeOther,
		"anslag":          ChapterTypeOther,
		"teaser":          ChapterTypeOther,
		"reality":         ChapterTypeOther,
		"studio":          ChapterTypeOther,
		"talk-show":       ChapterTypeOther,
		"presentasjon":    ChapterTypeOther,
		"seminar":         ChapterTypeOther,
		"reportasje":      ChapterTypeOther,
	}
)

var SongExtract = regexp.MustCompile("(FMB|HV) ?-? ?([0-9]+)")
var SongCollectionMap = map[string]string{
	"FMB": "AB",
	"HV":  "WOTL",
}

type Chapter struct {
	ChapterType    string
	Timestamp      float64
	Label          string
	Title          string
	Description    string
	SongCollection string
	SongNumber     string
	Highlight      bool
	Persons        []string
}

func (c *Client) GetChapterData(exportData *ExportData) ([]Chapter, error) {
	metaCache := map[string]*MetadataResult{}

	allChapters := map[string]*MetadataResult{}

	for _, clip := range exportData.Clips {
		if _, ok := metaCache[clip.VXID]; !ok {
			meta, err := c.GetMetadata(clip.VXID)
			if err != nil {
				return nil, err
			}
			metaCache[clip.VXID] = meta
		}

		sourceMeta := metaCache[clip.VXID]
		startTC := sourceMeta.Get(FieldStartTC, "0")
		tcStartSeconds, _ := TCToSeconds(startTC)

		// The result here is in TC of the original MEDIA.
		chapterMeta, err := c.GetChapterMeta(clip.VXID, clip.InSeconds+tcStartSeconds, clip.OutSeconds+tcStartSeconds)
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

			tcIn1, _ := TCToSeconds(data.Terse["title"][0].Start)
			tcOut1, _ := TCToSeconds(data.Terse["title"][0].End)

			tcIn2, _ := TCToSeconds(allChapters[title].Terse["title"][0].Start)
			tcOut2, _ := TCToSeconds(allChapters[title].Terse["title"][0].End)

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

	chapters := []Chapter{}
	for _, data := range allChapters {
		chapters = append(chapters, metaToChapter(data))
	}

	return chapters, nil
}

func metaToChapter(meta *MetadataResult) Chapter {
	out := Chapter{}

	out.Label = meta.Get(FieldTitle, "")
	out.Title = meta.Get(FieldTitle, "")
	start, _ := TCToSeconds(meta.Terse["title"][0].Start)
	out.Timestamp = start

	if chapterType, ok := ChapterTypeMap[meta.Get(FieldSubclipType, "")]; ok {
		out.ChapterType = chapterType.Value
	} else {
		out.ChapterType = ChapterTypeOther.Value
	}

	// This is more or less useless
	// out.Description = meta.Get(FieldDescription, "")

	out.Highlight = false // When do we set this?

	out.Persons = lo.Filter(meta.GetArray(FieldPersonsAppearing), func(p string, _ int) bool { return p != "" })

	if out.ChapterType == ChapterTypeSong.Value {
		match := SongExtract.FindStringSubmatch(strings.ToUpper(out.Label))
		if len(match) == 3 {
			out.SongCollection = match[1]
			out.SongCollection = match[2]
		}
	}

	return out
}

func (c *Client) GetChapterMeta(itemVXID string, inTc, outTc float64) (map[string]*MetadataResult, error) {
	inString := fmt.Sprintf("%.2f", inTc)
	outString := fmt.Sprintf("%.2f", outTc)

	url := fmt.Sprintf("%s/item/%s?content=metadata&terse=true&sampleRate=PAL&interval=%s-%s&group=Subclips", c.baseURL, itemVXID, inString, outString)

	resp, err := c.restyClient.R().
		SetResult(&MetadataResult{}).
		Get(url)

	if err != nil {
		return nil, err
	}

	metaResult := resp.Result().(*MetadataResult)

	clips := metaResult.SplitByClips()
	outClips := map[string]*MetadataResult{}
	for key, clip := range clips {

		if clip.Get(FieldExportAsChapter, "") != "export_as_chapter" {
			continue
		}

		for _, field := range clip.Terse {
			for _, value := range field {
				if valueStart, _ := TCToSeconds(value.Start); valueStart < inTc {
					value.Start = fmt.Sprintf("%.0f@PAL", inTc*25)
				}

				if valueEnd, _ := TCToSeconds(value.End); valueEnd > outTc {
					value.End = fmt.Sprintf("%.0f@PAL", outTc*25)
				}

			}
		}

		outClips[key] = clip
	}

	return outClips, nil
}
