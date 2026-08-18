package utils

import (
	"github.com/bcc-code/bcc-media-flows/languages"
	"github.com/samber/lo"
	"sort"
)

func LanguageKeysToOrderedLanguages(keys []string) languages.LanguageList {
	// Do we want this to fail the job if key doesn't exist? Will panic.
	languages := languages.LanguageList(lo.Map(keys, func(key string, _ int) languages.Language {
		return languages.LanguagesByISO[key]
	}))

	// Sort languages by priority
	sort.Sort(languages)
	return languages
}
