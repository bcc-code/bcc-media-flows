package vsapi_test

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/bcc-code/bccm-flows/services/vidispine/vsapi"
	"github.com/bcc-code/bccm-flows/services/vidispine/vscommon"
	"github.com/stretchr/testify/assert"
)

func Test_GetInOut_Asset(t *testing.T) {
	testData, err := os.ReadFile("testdata/assets/no-subclip.json")
	assert.NoError(t, err)

	m := vsapi.MetadataResult{}
	err = json.Unmarshal(testData, &m)
	assert.NoError(t, err)

	meta := m.SplitByClips()

	in, out, err := meta[vsapi.OriginalClip].GetInOut("")
	assert.NoError(t, err)
	assert.Equal(t, 0.0, in)
	assert.Equal(t, 7012.0, out)
}

func Test_GetInOut_Subclip(t *testing.T) {
	testData, err := os.ReadFile("testdata/assets/subclip.json")
	assert.NoError(t, err)

	m := vsapi.MetadataResult{}
	err = json.Unmarshal(testData, &m)
	assert.NoError(t, err)

	meta := m.SplitByClips()

	tcStart := meta[vsapi.OriginalClip].Get(vscommon.FieldStartTC, "0@PAL")

	in, out, err := meta["John Doe - Speech"].GetInOut(tcStart)
	assert.NoError(t, err)
	assert.Equal(t, 1172.800000000003, in)
	assert.Equal(t, 3335.479999999996, out)
}

func Test_GetInOut_SubclipErr(t *testing.T) {
	testData, err := os.ReadFile("testdata/assets/subclip.json")
	assert.NoError(t, err)

	m := vsapi.MetadataResult{}
	err = json.Unmarshal(testData, &m)
	assert.NoError(t, err)

	meta := m.SplitByClips()

	in, out, err := meta["John Doe - Speech"].GetInOut("0")
	assert.Error(t, err)
	assert.Equal(t, 0.0, in)
	assert.Equal(t, 0.0, out)
}
