package utils

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_GetSiblingFolder(t *testing.T) {
	//test of GetSiblingFolder

	path := "/mnt/isilon/Transcoding/ProRes422HQ_Native/in/MFTB_2022_beauty_night_0004.MP4"

	path, err := GetSiblingFolder(path, "sibling")

	assert.Nil(t, err)
	assert.Equal(t, "/mnt/isilon/Transcoding/ProRes422HQ_Native/sibling", path)
}
