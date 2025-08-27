package paths

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
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

func Test_Lucid(t *testing.T) {
	pathString := "/mnt/isilon/system/multitrack/Ingest/tempFraBrunstad/Felles/Opptak1/lkajhdwid-323.wav"

	path, err := Parse(pathString)

	assert.Nil(t, err)

	assert.Equal(t, IsilonDrive, path.Drive)
	assert.Equal(t, "system/multitrack/Ingest/tempFraBrunstad/Felles/Opptak1/lkajhdwid-323.wav", path.Path)

	assert.Equal(t, "isilon:isilon/system/multitrack/Ingest/tempFraBrunstad/Felles/Opptak1/lkajhdwid-323.wav", path.Rclone())

	lucidPath := Path{
		Drive: LucidLinkDrive,
		Path:  strings.Replace(path.Dir().Path, "system/multitrack/Ingest/tempFraBrunstad", "", 1),
	}

	lucidPath = lucidPath.Append(path.Base()).Prepend("/tesing/test/test")

	assert.Equal(t, "lucid:lucidlink/tesing/test/test/Felles/Opptak1/lkajhdwid-323.wav", lucidPath.Rclone())
}

func Test_FileCatalyst(t *testing.T) {
	pathString := "/mnt/filecatalyst/ingestgrow/MDTES01_MU1.mxf"

	path, err := Parse(pathString)

	assert.Nil(t, err)

	assert.Equal(t, FileCatalystDrive, path.Drive)
	assert.Equal(t, "ingestgrow/MDTES01_MU1.mxf", path.Path)

	assert.Equal(t, "/mnt/filecatalyst/ingestgrow/MDTES01_MU1.mxf", path.Local())
}

func Test_InvalidPath(t *testing.T) {
	pathString := "/an/invalid/path"
	pathRes, err := Parse(pathString)
	assert.Empty(t, pathRes)
	assert.Error(t, err)
}

func Test_Massive(t *testing.T) {
	pathString := "s3prod:/massiveio-bccm/upload/PACE - 202508251231"
	path, err := Parse(pathString)

	assert.Nil(t, err)
	assert.Equal(t, MassiveIngestDrive, path.Drive)
	assert.Equal(t, "upload/PACE - 202508251231", path.Path)
}
