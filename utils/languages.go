package utils

import (
	"fmt"
	"sort"

	"github.com/bcc-code/bcc-media-flows/languages"
	"github.com/samber/lo"
)

func LanguageKeysToOrderedLanguages(keys []string) languages.LanguageList {
	langs := languages.LanguageList(lo.Map(keys, func(key string, _ int) languages.Language {
		lang, ok := languages.LanguagesByISO[key]
		if !ok {
			panic(fmt.Sprintf("unknown language key: %q", key))
		}
		return lang
	}))

	// Sort languages by priority
	sort.Sort(langs)
	return langs
}
