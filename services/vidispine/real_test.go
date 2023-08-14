package vidispine_test

import (
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
