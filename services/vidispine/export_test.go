package vidispine_test

// This test will only run if the build tag testLive is set.
// To run this test, run:
// go test -tags testLive

// Be careful, this will manipulate data in Vidispine.

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"os"
	"testing"

	"github.com/bcc-code/bcc-media-flows/services/vidispine"
	"github.com/bcc-code/bcc-media-flows/services/vidispine/vsapi"
	"github.com/bcc-code/bcc-media-flows/services/vidispine/vsmock"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func fromJSONFile[T any](o T, file string) {
	data, err := os.ReadFile(file)
	if err != nil {
		panic(err)
	}

	err = json.Unmarshal(data, o)
	if err != nil {
		panic(err)
	}
}

func fromXMLFile[T any](o T, file string) {
	data, err := os.ReadFile(file)
	if err != nil {
		panic(err)
	}

	err = xml.Unmarshal(data, o)
	if err != nil {
		panic(err)
	}
}

func expectGetShape(vs *vsmock.MockClient, vxID string, count int) {
	shape := &vsapi.ShapeResult{}
	fromJSONFile(shape, fmt.Sprintf("testdata/get_shapes/%s.json", vxID))
	vs.EXPECT().GetShapes(vxID).Return(shape, nil).Times(count)
}

func expectGetMetadata(vs *vsmock.MockClient, vxID string, count int) {
	meta := &vsapi.MetadataResult{}
	fromJSONFile(meta, fmt.Sprintf("testdata/get_metadata/%s.json", vxID))
	vs.EXPECT().GetMetadata(vxID).Return(meta, nil).Times(count)
}

func expectGetSequence(vs *vsmock.MockClient, vxID string, count int) {
	seq := &vsapi.SequenceDocument{}
	fromXMLFile(seq, fmt.Sprintf("testdata/get_sequence/%s.xml", vxID))
	vs.EXPECT().GetSequence(vxID).Return(seq, nil).Times(count)
}

func Test_GetDataForExportSEQ(t *testing.T) {
	ctrl := gomock.NewController(t)
	vsClient := vsmock.NewMockClient(ctrl)

	expectGetMetadata(vsClient, "VX-431566", 1)
	expectGetMetadata(vsClient, "VX-431552", 45)

	expectGetSequence(vsClient, "VX-431566", 1)

	expectGetShape(vsClient, "VX-431552", 6)
	expectGetShape(vsClient, "VX-431547", 3)
	expectGetShape(vsClient, "VX-431555", 3)
	expectGetShape(vsClient, "VX-431548", 3)
	expectGetShape(vsClient, "VX-431558", 3)
	expectGetShape(vsClient, "VX-431550", 3)
	expectGetShape(vsClient, "VX-431556", 3)
	expectGetShape(vsClient, "VX-431557", 3)
	expectGetShape(vsClient, "VX-431554", 3)
	expectGetShape(vsClient, "VX-431551", 3)
	expectGetShape(vsClient, "VX-431560", 3)
	expectGetShape(vsClient, "VX-431549", 3)
	expectGetShape(vsClient, "VX-431553", 3)
	expectGetShape(vsClient, "VX-431561", 3)
	expectGetShape(vsClient, "VX-431562", 3)
	expectGetShape(vsClient, "VX-431559", 3)

	c := vsClient

	// SEQ - Chapters
	expected := &vidispine.ExportData{}
	fromJSONFile(expected, "testdata/GetDataForExport/VX-431566.json")

	res, err := vidispine.GetDataForExport(c, "VX-431566", nil, nil, "")
	assert.NoError(t, err)
	assert.Equal(t, expected, res)

	ctrl.Finish()
}

func Test_GetDataForExportEmbeddedAudio(t *testing.T) {
	ctrl := gomock.NewController(t)
	vsClient := vsmock.NewMockClient(ctrl)

	expectGetMetadata(vsClient, "VX-464406", 1)

	expectGetSequence(vsClient, "VX-464406", 1)

	expectGetShape(vsClient, "VX-464402", 3)

	// SEQ - Embedded audio
	expected := &vidispine.ExportData{}
	fromJSONFile(expected, "testdata/GetDataForExport/VX-464406.json")

	res, err := vidispine.GetDataForExport(vsClient, "VX-464406", nil, nil, "")
	assert.NoError(t, err)
	assert.Equal(t, expected, res)
}

func Test_GetDataForExportAsset(t *testing.T) {
	ctrl := gomock.NewController(t)
	vsClient := vsmock.NewMockClient(ctrl)

	expectGetMetadata(vsClient, "VX-464458", 1)

	expectGetShape(vsClient, "VX-464458", 3)

	// Asset
	expected := &vidispine.ExportData{}
	fromJSONFile(expected, "testdata/GetDataForExport/VX-464458.json")

	res, err := vidispine.GetDataForExport(vsClient, "VX-464458", nil, nil, "")
	assert.NoError(t, err)
	assert.Equal(t, expected, res)
}

func Test_GetDataForExportSubclip(t *testing.T) {
	ctrl := gomock.NewController(t)
	vsClient := vsmock.NewMockClient(ctrl)

	expectGetMetadata(vsClient, "VX-460824", 1)

	expectGetShape(vsClient, "VX-460824", 3)

	expected := &vidispine.ExportData{}
	fromJSONFile(expected, "testdata/GetDataForExport/VX-460824.json")

	res, err := vidispine.GetDataForExport(vsClient, "VX-460824", nil, nil, "")
	assert.NoError(t, err)
	assert.Equal(t, expected, res)
}

func Test_GetDataForExportSubtitles(t *testing.T) {
	ctrl := gomock.NewController(t)
	vsClient := vsmock.NewMockClient(ctrl)

	expectGetMetadata(vsClient, "VX-447459", 1)
	expectGetMetadata(vsClient, "VX-447219", 13)
	expectGetMetadata(vsClient, "VX-439354", 13)

	expectGetSequence(vsClient, "VX-447459", 1)

	expectGetShape(vsClient, "VX-447219", 2)
	expectGetShape(vsClient, "VX-439354", 2)
	expectGetShape(vsClient, "VX-447207", 1)
	expectGetShape(vsClient, "VX-447208", 1)
	expectGetShape(vsClient, "VX-447209", 1)
	expectGetShape(vsClient, "VX-447210", 1)
	expectGetShape(vsClient, "VX-447211", 1)
	expectGetShape(vsClient, "VX-447212", 1)
	expectGetShape(vsClient, "VX-447213", 1)
	expectGetShape(vsClient, "VX-447214", 1)
	expectGetShape(vsClient, "VX-447215", 1)
	expectGetShape(vsClient, "VX-447216", 1)
	expectGetShape(vsClient, "VX-447217", 1)
	expectGetShape(vsClient, "VX-447206", 1)
	expectGetShape(vsClient, "VX-447499", 1)

	expected := &vidispine.ExportData{}
	fromJSONFile(expected, "testdata/GetDataForExport/VX-447459.json")

	res, err := vidispine.GetDataForExport(vsClient, "VX-447459", nil, nil, "")
	assert.NoError(t, err)
	assert.Equal(t, expected, res)
}

/*
func Test_GetChapterData(t *testing.T) {
	ctrl := gomock.NewController(t)
	vsClient := vsmock.NewMockVSClient(ctrl)

	expectGetMetadata(vsClient, "VX-431566", 1)


	c := vidispine.NewVidispineService(vsClient)

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
		spew.Dump(chapters)
	}

}*/
