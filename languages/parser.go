package languages

import "errors"

var ErrLanguageParsingFailed = errors.New("uanable to parse language code")

func ParseLanguageCode(langCode string) (Language, error) {

	if lang, ok := LanguagesByISO[langCode]; ok {
		return lang, nil
	}

	if lang, ok := LanguagesByISOTwoLetter[langCode]; ok {
		return lang, nil
	}

	if lang, ok := LanguageByBMM[langCode]; ok {
		return lang, nil
	}

	return Language{}, ErrLanguageParsingFailed
}

func MustParseLanguageCode(langCode string) Language {
	l, err := ParseLanguageCode(langCode)
	if err != nil {
		panic(err)
	}

	return l
}
