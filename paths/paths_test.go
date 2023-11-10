package paths

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

func Test_ParsePath(t *testing.T) {
	pathString := "/mnt/isilon/test.xml"

	path, err := Parse(pathString)

	assert.Nil(t, err)

	assert.Equal(t, IsilonDrive, path.Drive)
	assert.Equal(t, "test.xml", path.Path)

	assert.Equal(t, "isilon:isilon/test.xml", path.Rclone())
}
