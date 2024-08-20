package export

import (
	"testing"

	"github.com/bcc-code/bcc-media-flows/utils"

	"github.com/davecgh/go-spew/spew"
)

func Test_getQualitiesWithLanguages(t *testing.T) {
	l := assignLanguagesToResolutions([]string{"en", "no"}, []utils.Resolution{
		{
			Width:  1920,
			Height: 1080,
			File:   true,
		},
		{
			Width:  1280,
			Height: 720,
			File:   true,
		},
		{
			Width:  640,
			Height: 360,
			File:   true,
		},
	})
	spew.Dump(l)
}
