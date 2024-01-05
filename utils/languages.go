package utils

import (
	bccmflows "github.com/bcc-code/bcc-media-flows"
	"github.com/samber/lo"
	"sort"
)

func LanguageKeysToOrderedLanguages(keys []string) bccmflows.LanguageList {
	// Do we want this to fail the job if key doesn't exist? Will panic.
	languages := bccmflows.LanguageList(lo.Map(keys, func(key string, _ int) bccmflows.Language {
		return bccmflows.LanguagesByISO[key]
	}))

	// Sort languages by priority
	sort.Sort(languages)
	return languages
}
