package export

import (
	"testing"

	"github.com/bcc-code/bcc-media-flows/utils"
)

func Test_getQualitiesWithLanguages(t *testing.T) {
	l := assignLanguagesToResolutions([]string{"en", "no"}, []utils.Resolution{
		{
			Width:  1920,
			Height: 1080,
			IsFile: true,
		},
		{
			Width:  1280,
			Height: 720,
			IsFile: true,
		},
		{
			Width:  640,
			Height: 360,
			IsFile: true,
		},
	})

	// Assert that the correct number of resolutions is returned
	if len(l) != 3 {
		t.Errorf("expected 3 resolutions, got %d", len(l))
	}

	// Assert that the first resolution has both languages
	if len(l[0].Languages) != 2 {
		t.Errorf("expected 2 languages for first resolution, got %d", len(l[0].Languages))
	}

	// Assert that the next resolutions have 0 languages (since only 2 provided)
	if len(l[1].Languages) != 0 {
		t.Errorf("expected 0 languages for second resolution, got %d", len(l[1].Languages))
	}
	if len(l[2].Languages) != 0 {
		t.Errorf("expected 0 languages for third resolution, got %d", len(l[2].Languages))
	}
}

func Test_assignLanguagesToResolutions_noLanguages(t *testing.T) {
	l := assignLanguagesToResolutions([]string{}, []utils.Resolution{
		{Width: 1920, Height: 1080, IsFile: true},
		{Width: 1280, Height: 720, IsFile: true},
	})

	if len(l) != 2 {
		t.Errorf("expected 2 resolutions, got %d", len(l))
	}
	for i, res := range l {
		if len(res.Languages) != 0 {
			t.Errorf("expected 0 languages for resolution %d, got %d", i, len(res.Languages))
		}
	}
}

func Test_assignLanguagesToResolutions_manyLanguages(t *testing.T) {
	langs := []string{"en", "no", "de", "fr", "es", "it", "ru", "sv", "da", "fi"}
	l := assignLanguagesToResolutions(langs, []utils.Resolution{
		{Width: 1920, Height: 1080, IsFile: true},
		{Width: 1280, Height: 720, IsFile: true},
	})

	if len(l) != 2 {
		t.Errorf("expected 2 resolutions, got %d", len(l))
	}
	// Only 8 languages max per resolution, so first gets 8, second gets 2
	if len(l[0].Languages) != 8 {
		t.Errorf("expected 8 languages for first resolution, got %d", len(l[0].Languages))
	}
	if len(l[1].Languages) != 2 {
		t.Errorf("expected 2 languages for second resolution, got %d", len(l[1].Languages))
	}
}
