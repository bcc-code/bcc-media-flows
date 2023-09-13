//go:build testLive

package vidispine_test

// This test will only run if the build tag testLive is set.
// To run this test, run:
// go test -tags testLive

// Be careful, this will manipulate data in Vidispine.

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/bcc-code/bccm-flows/services/vidispine"
	"github.com/davecgh/go-spew/spew"
	"github.com/stretchr/testify/assert"
)

func getClient() *vidispine.Client {
	return vidispine.NewClient(
		os.Getenv("VIDISPINE_BASE_URL"),
		os.Getenv("VIDISPINE_USERNAME"),
		os.Getenv("VIDISPINE_PASSWORD"),
	)
}

func Test_GetVSMetadata(t *testing.T) {
	c := getClient()
	res, err := c.GetMetadata("VX-462592")
	assert.NoError(t, err)
	assert.NotNil(t, res)
}

func Test_GetVSShapes(t *testing.T) {
	c := getClient()
	res, err := c.GetShapes("VX-462592")
	assert.NoError(t, err)

	shape := res.GetShape("original")
	assert.NotNil(t, shape)
	//spew.Dump(shape.ContainerComponent.File[0].Path)
}

func Test_CreatePlaceholder(t *testing.T) {
	c := getClient()
	placeholderID, err := c.CreatePlaceholder(vidispine.PLACEHOLDER_TYPE_MASTER, "test", "matjaz.debelak@bcc.no")
	assert.NoError(t, err)
	assert.NotEmpty(t, placeholderID)
}

func Test_AddShapeToItem(t *testing.T) {
	c := getClient()
	out, err := c.AddShapeToItem("lowimage", "VX-463136", "VX-1458094")
	assert.NoError(t, err)
	spew.Dump(out)
	assert.NotEmpty(t, out)
}

func Test_AddFileToPlaceholder(t *testing.T) {
	c := getClient()

	url := c.AddFileToPlaceholder("VX-ITEM", "VX-FILE", "tag", vidispine.FILE_STATE_CLOSED)
	assert.Equal(t, "http://10.12.128.15:8080/import/placeholder/VX-ITEM/container?fileId=VX-FILE&growing=false&tag=tag", url)

	url = c.AddFileToPlaceholder("VX-ITEM", "VX-FILE", "", vidispine.FILE_STATE_CLOSED)
	assert.Equal(t, "http://10.12.128.15:8080/import/placeholder/VX-ITEM/container?fileId=VX-FILE&growing=false", url)

	url = c.AddFileToPlaceholder("VX-ITEM", "VX-FILE", "tag", vidispine.FILE_STATE_OPEN)
	assert.Equal(t, "http://10.12.128.15:8080/import/placeholder/VX-ITEM/container?fastStartLength=7200&fileId=VX-FILE&growing=true&jobmetadata=portal_groups%3AStringArray%253dAdmin&overrideFastStart=true&requireFastStart=true&settings=VX-76&tag=tag", url)

}

func Test_GetDataForExport(t *testing.T) {
	c := getClient()
	var err error

	// SEQ - Chapters
	res, err := c.GetDataForExport("VX-431566")
	assert.NoError(t, err)
	j, _ := json.MarshalIndent(res, "", "  ")
	fmt.Printf("%s", j)

	// SEQ - Embedded audio
	_, err = c.GetDataForExport("VX-464406")
	assert.NoError(t, err)

	// Asset - Master, Embedded Audio, Subtitles - Should error
	// _, err = c.GetDataForExport("VX-447219")

	// SEQ - Master, Embedded Audio, Subtitles
	//_, err = c.GetDataForExport("VX-447459", true)
	//assert.NoError(t, err)

	// SEQ - Related Audio
	//err = c.GetDataForExport("VX-464448")

	// SEQ - Related Audio
	//err = c.GetDataForExport("VX-464480")

	// Asset
	//c.GetDataForExport("VX-464458")

	// Subclip
	//c.GetDataForExport("VX-460824")
}

func Test_GetMetaChapterData(t *testing.T) {
	c := getClient()
	res, err := c.GetChapterMeta("VX-431552", 52171.48, 52280.0)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(res))
}

func Test_GetChapterData(t *testing.T) {
	c := getClient()

	testVXIDs := []string{
		"VX-431566",
		"VX-411326",
		"VX-410884",
		"VX-467749",
	}

	for _, vxid := range testVXIDs {
		spew.Dump(vxid)
		exportData, err := c.GetDataForExport(vxid)
		assert.NoError(t, err)
		assert.NotNil(t, exportData)

		chapters, err := c.GetChapterData(exportData)
		assert.NoError(t, err)
		assert.NotNil(t, chapters)
		// spew.Dump(lo.Filter(chapters, func(c vidispine.Chapter, _ int) bool { return c.ChapterType == "song" }))
		spew.Dump(chapters)
	}

}
