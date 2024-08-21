package export

import (
	"testing"

	"github.com/bcc-code/bcc-media-flows/utils"

	"github.com/davecgh/go-spew/spew"
)

func Test_getQualitiesWithLanguages(t *testing.T) {
	l := getQualitiesWithLanguages([]string{"en", "no"}, []utils.Resolution{
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
	spew.Dump(l)
}
