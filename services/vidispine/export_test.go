package vidispine_test

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/bcc-code/bccm-flows/services/vidispine"
	"github.com/stretchr/testify/assert"
)

func Test_TCToSeconds(t *testing.T) {
	testData := []struct {
		in    string
		out   float64
		isErr bool
	}{
		{"0@PAL", 0, false},
		{"25@PAL", 1.0, false},
		{"25000@PAL", 1000.0, false},
		{"25000@NTSC", 0.0, true},
	}

	for _, td := range testData {
		out, err := vidispine.TCToSeconds(td.in)
		assert.Equal(t, td.out, out)
		assert.Equal(t, td.isErr, err != nil)
	}
}

func Test_GetInOut_Asset(t *testing.T) {
	testData, err := os.ReadFile("testdata/asset-no-subclip.json")
	assert.NoError(t, err)

	m := vidispine.MetadataResult{}
	err = json.Unmarshal(testData, &m)
	assert.NoError(t, err)

	meta := m.SplitByClips()

	in, out, err := meta[vidispine.OriginalClip].GetInOut("")
	assert.NoError(t, err)
	assert.Equal(t, 0.0, in)
	assert.Equal(t, 7012.0, out)
}

func Test_GetInOut_Subclip(t *testing.T) {
	testData, err := os.ReadFile("testdata/subclip.json")
	assert.NoError(t, err)

	m := vidispine.MetadataResult{}
	err = json.Unmarshal(testData, &m)
	assert.NoError(t, err)

	meta := m.SplitByClips()

	tcStart := meta[vidispine.OriginalClip].Get("startTimeCode", "0@PAL")

	in, out, err := meta["John Doe - Speech"].GetInOut(tcStart)
	assert.NoError(t, err)
	assert.Equal(t, 1172.800000000003, in)
	assert.Equal(t, 3335.479999999996, out)
}

func Test_GetInOut_SubclipErr(t *testing.T) {
	testData, err := os.ReadFile("testdata/subclip.json")
	assert.NoError(t, err)

	m := vidispine.MetadataResult{}
	err = json.Unmarshal(testData, &m)
	assert.NoError(t, err)

	meta := m.SplitByClips()

	in, out, err := meta["John Doe - Speech"].GetInOut("0")
	assert.Error(t, err)
	assert.Equal(t, 0.0, in)
	assert.Equal(t, 0.0, out)
}
