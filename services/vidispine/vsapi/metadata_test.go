package vsapi

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/bcc-code/bcc-media-flows/services/vidispine/vscommon"
	"github.com/stretchr/testify/assert"
)

func Test_GetInOut_Asset(t *testing.T) {
	testData, err := os.ReadFile("testdata/assets/no-subclip.json")
	assert.NoError(t, err)

	m := MetadataResult{}
	err = json.Unmarshal(testData, &m)
	assert.NoError(t, err)

	meta := m.SplitByClips()

	in, out, err := meta[OriginalClip].GetInOut("")
	assert.NoError(t, err)
	assert.Equal(t, 0.0, in)
	assert.Equal(t, 7012.0, out)
}

func Test_GetInOut_Subclip(t *testing.T) {
	testData, err := os.ReadFile("testdata/assets/subclip.json")
	assert.NoError(t, err)

	m := MetadataResult{}
	err = json.Unmarshal(testData, &m)
	assert.NoError(t, err)

	meta := m.SplitByClips()

	tcStart := meta[OriginalClip].Get(vscommon.FieldStartTC, "0@PAL")

	in, out, err := meta["John Doe - Speech"].GetInOut(tcStart)
	assert.NoError(t, err)
	assert.Equal(t, 1172.800000000003, in)
	assert.Equal(t, 3335.479999999996, out)
}

func Test_GetInOut_SubclipErr(t *testing.T) {
	testData, err := os.ReadFile("testdata/assets/subclip.json")
	assert.NoError(t, err)

	m := MetadataResult{}
	err = json.Unmarshal(testData, &m)
	assert.NoError(t, err)

	meta := m.SplitByClips()

	in, out, err := meta["John Doe - Speech"].GetInOut("0")
	assert.Error(t, err)
	assert.Equal(t, 0.0, in)
	assert.Equal(t, 0.0, out)
}

func Test_GenerateMetUpdateXML(t *testing.T) {
	buf, _ := createSetItemMetadataFieldXml(SetItemMetadataFieldParams{
		StartTC: "-INF",
		EndTC:   "+INF",
		GroupID: "System",
		Key:     "portal_mf442906",
		Value:   "VX-480938",
	})

	print(buf.String())
	expected := `<?xml version="1.0"?>
<MetadataDocument xmlns="http://xml.vidispine.com/schema/vidispine">
	<timespan start="-INF" end="+INF">
		
		<group>
			<name>System</name>
		
		<field>
			<name>portal_mf442906</name>
			
				<value>VX-480938</value>
			
		</field>
		
		</group>
		
	</timespan>
</MetadataDocument>`
	assert.Equal(t, expected, buf.String())
}

func Test_GenerateMetUpdateWithTCXML(t *testing.T) {
	buf, _ := createSetItemMetadataFieldXml(SetItemMetadataFieldParams{
		StartTC: "arbitraryValue1",
		EndTC:   "arbitraryValue2",
		Key:     "portal_mf442906",
		Value:   "VX-480938",
	})

	print(buf.String())
	expected := `<?xml version="1.0"?>
<MetadataDocument xmlns="http://xml.vidispine.com/schema/vidispine">
	<timespan start="arbitraryValue1" end="arbitraryValue2">
		
		<field>
			<name>portal_mf442906</name>
			
				<value>VX-480938</value>
			
		</field>
		
	</timespan>
</MetadataDocument>`
	assert.Equal(t, expected, buf.String())
}
