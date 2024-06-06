package vidispine

import (
	"fmt"
	"math"
	"slices"

	"github.com/bcc-code/bcc-media-flows/services/vidispine/vsapi"
	"github.com/bcc-code/bcc-media-flows/services/vidispine/vscommon"
	"github.com/samber/lo"
)

type GetChapterMetaResult struct {
	Meta          *vsapi.MetadataResult
	OriginalStart float64
}

// GetChapterMetaForClips will return all chapters for the given clips.
//
// Clips might be a part of a sequence, so this function will also convert the timecodes
// to be relative to the sequence.
func GetChapterMetaForClips(client Client, clips []*Clip) ([]*GetChapterMetaResult, error) {
	metaCache := map[string]*vsapi.MetadataResult{}
	allChapters := map[string][]*GetChapterMetaResult{}

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

			// find overlapping chapters with the same title
			other, found := lo.Find(allChapters[title], func(chapter *GetChapterMetaResult) bool {
				return isOverlapping(chapter.Meta.Terse, data.Terse)
			})
			if found {
				other.Meta.Terse = mergeTerseTimecodes(other.Meta.Terse, data.Terse)
				o, err := vscommon.TCToSeconds(data.Terse["title"][0].Start)
				if err != nil {
					other.OriginalStart = o
				}
			}

			allChapters[title] = append(allChapters[title], &GetChapterMetaResult{
				Meta:          data,
				OriginalStart: clip.InSeconds,
			})
		}
	}

	var out []*GetChapterMetaResult
	for _, chapters := range allChapters {
		out = append(out, chapters...)
	}
	slices.SortFunc(out, func(a, b *GetChapterMetaResult) int {
		return int(a.OriginalStart - b.OriginalStart)
	})
	return out, nil
}

func isOverlapping(terseA, terseB map[string][]*vsapi.MetadataField) bool {
	tcIn1, _ := vscommon.TCToSeconds(terseA["title"][0].Start)
	tcOut1, _ := vscommon.TCToSeconds(terseA["title"][0].End)

	tcIn2, _ := vscommon.TCToSeconds(terseB["title"][0].Start)
	tcOut2, _ := vscommon.TCToSeconds(terseB["title"][0].End)

	return tcIn1 < tcOut2 && tcOut1 > tcIn2
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
