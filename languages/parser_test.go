package languages

import (
	"errors"
	"testing"
)

func TestParseLanguageCode(t *testing.T) {
	lang, err := ParseLanguageCode("nor")
	if err != nil {
		t.Fatalf("ParseLanguageCode(\"nor\") returned error: %v", err)
	}
	if lang.LanguageNameSystem != "Norwegian" {
		t.Fatalf("expected Norwegian, got %s", lang.LanguageNameSystem)
	}

	for _, code := range []string{"", "xx"} {
		_, err := ParseLanguageCode(code)
		if !errors.Is(err, ErrLanguageParsingFailed) {
			t.Fatalf("ParseLanguageCode(%q): expected ErrLanguageParsingFailed, got %v", code, err)
		}
	}
}
