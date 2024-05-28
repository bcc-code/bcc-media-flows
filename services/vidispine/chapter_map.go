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
	chapterTypeMap = map[string]pcommon.ChapterType{
		"sang":        pcommon.ChapterTypeSong,
		"musikkvideo": pcommon.ChapterTypeSong,
		"musikal":     pcommon.ChapterTypeSong,
		"tale":        pcommon.ChapterTypeSpeech,
		"appelle":     pcommon.ChapterTypeSpeech,
		"vitnesbyrd":  pcommon.ChapterTypeTestimony,
		"singalong":   pcommon.ChapterTypeSingAlong,
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
	if chapterType, ok := chapterTypeMap[vsChapterType]; ok {
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
