package vidispine

import (
	"regexp"

	pcommon "github.com/bcc-code/bcc-media-platform/backend/common"
)

var SongExtract = regexp.MustCompile("(FMB|HV) ?-? ?([0-9]+)")
var SongCollectionMap = map[string]string{
	"FMB": "AB",
	"HV":  "WOTL",
}
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

var chapterTypesToFilterOut = []string{
	"end-credit",
	"tydning",
	"bnn",
	"frsending",
	"ettersending",
	"programleder",
}

func mapSubclipType(vsChapterType string) pcommon.ChapterType {
	if chapterType, ok := ChapterTypeMap[vsChapterType]; ok {
		return chapterType
	}
	return pcommon.ChapterTypeOther
}

func findBestChapterType(subclipTypes []string) (string, *pcommon.ChapterType) {
	if len(subclipTypes) > 1 {
		for _, prioritizedType := range []pcommon.ChapterType{
			pcommon.ChapterTypeOther,
			pcommon.ChapterTypeInterview,
			pcommon.ChapterTypeTheme,
			pcommon.ChapterTypeSpeech,
			pcommon.ChapterTypeSingAlong,
			pcommon.ChapterTypeSong,
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
