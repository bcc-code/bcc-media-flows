package export

import (
	"testing"

	"github.com/davecgh/go-spew/spew"
)

func Test_getQualitiesWithLanguages(t *testing.T) {
	l := getQualitiesWithLanguages([]string{"en", "no"}, []Resolution{
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
