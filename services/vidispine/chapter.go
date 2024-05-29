package vidispine

import (
	"fmt"
	"math"
	"strings"

	"github.com/bcc-code/bcc-media-platform/backend/asset"
	pcommon "github.com/bcc-code/bcc-media-platform/backend/common"

	"github.com/bcc-code/bcc-media-flows/services/vidispine/vsapi"
	"github.com/bcc-code/bcc-media-flows/services/vidispine/vscommon"
	"github.com/samber/lo"
)

type GetChapterMetaResult struct {
	AllChapters   map[string]*vsapi.MetadataResult
	OriginalStart map[string]float64
}

// GetChapterMetaForClips will return all chapters for the given clips.
//
// Clips might be a part of a sequence, and this function will also convert the timecodes
// to be relative to the sequence.
//
//	It will also merge chapters with the same title, so that the in and out points are the earliest and latest.
func GetChapterMetaForClips(client Client, clips []*Clip) (*GetChapterMetaResult, error) {
	metaCache := map[string]*vsapi.MetadataResult{}
	allChapters := map[string]*vsapi.MetadataResult{}
	originalStart := map[string]float64{}

	for _, clip := range clips {
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

			chapter, exists := allChapters[title]
			if !exists {
				allChapters[title] = data
				originalStart[title] = clip.InSeconds
				continue
			}

			chapter.Terse = mergeTerseTimecodes(chapter.Terse, data.Terse)
			o, _ := vscommon.TCToSeconds(chapter.Terse["title"][0].Start)
			originalStart[title] = o
		}
	}

	return &GetChapterMetaResult{
		AllChapters:   allChapters,
		OriginalStart: originalStart,
	}, nil
}

// This chapter already exists, so we need to merge the data.
// Since the source is the same the only diff is the in and out point
// i.e. we only need the earlies in and latest out point on all values
func mergeTerseTimecodes(terseA, terseB map[string][]*vsapi.MetadataField) map[string][]*vsapi.MetadataField {
	tcIn1, _ := vscommon.TCToSeconds(terseA["title"][0].Start)
	tcOut1, _ := vscommon.TCToSeconds(terseA["title"][0].End)

	tcIn2, _ := vscommon.TCToSeconds(terseB["title"][0].Start)
	tcOut2, _ := vscommon.TCToSeconds(terseB["title"][0].End)

	newIn := math.Min(tcIn1, tcIn2)
	newOut := math.Max(tcOut1, tcOut2)

	for name := range terseB {
		for i := range terseB[name] {
			terseB[name][i].Start = fmt.Sprintf("%.0f@PAL", newIn*25)
			terseB[name][i].End = fmt.Sprintf("%.0f@PAL", newOut*25)
		}
	}

	return terseB
}

func GetTimedMetadataChapters(client Client, clips []*Clip) ([]asset.TimedMetadata, error) {
	vsChapters, err := GetChapterMetaForClips(client, clips)
	if err != nil {
		return nil, err
	}

	var chapters []asset.TimedMetadata
	for _, data := range vsChapters.AllChapters {
		chapter, keep := metaToChapter(data)
		if !keep {
			continue
		}
		if chapter.Timestamp == 0 {
			chapter.Timestamp = vsChapters.OriginalStart[data.Get(vscommon.FieldTitle, "")]
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
	if lo.Contains(chapterTypesToFilterOut, subclipTypes[0]) {
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
