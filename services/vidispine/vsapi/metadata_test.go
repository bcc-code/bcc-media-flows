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

func Test_GetInOut_EmptyTitle(t *testing.T) {
	m := MetadataResult{
		Terse: map[string][]*MetadataField{
			vscommon.FieldTitle.Value: {},
		},
	}

	in, out, err := m.GetInOut("")
	assert.EqualError(t, err, "Missing title")
	assert.Equal(t, 0.0, in)
	assert.Equal(t, 0.0, out)
}

func Test_GetInOut_MissingTitle(t *testing.T) {
	m := MetadataResult{Terse: map[string][]*MetadataField{}}

	in, out, err := m.GetInOut("")
	assert.EqualError(t, err, "Missing title")
	assert.Equal(t, 0.0, in)
	assert.Equal(t, 0.0, out)
}

func Test_GenerateMetUpdateXML(t *testing.T) {
	buf, _ := createSetItemMetadataFieldXml(xmlSetItemMetadataFieldParams{
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
	buf, _ := createSetItemMetadataFieldXml(xmlSetItemMetadataFieldParams{
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

func Test_MetadataDocumentJSON_GroupInstances(t *testing.T) {
	// MetadataListDocument envelope with nested groups.
	listDoc := `{"item":[{"id":"VX-1","metadata":{"timespan":[
		{"start":"-INF","end":"+INF","group":[
			{"uuid":"uuid-1","name":"stl_subtitle"},
			{"uuid":"uuid-2","name":"Subclips","group":[{"uuid":"uuid-3","name":"stl_subtitle"}]}
		]},
		{"start":"0@PAL","end":"250@PAL","group":[{"uuid":"uuid-4","name":"stl_subtitle"}]}
	]}}]}`

	doc := metadataDocumentJSON{}
	assert.NoError(t, json.Unmarshal([]byte(listDoc), &doc))

	timespans := doc.Timespan
	for _, item := range doc.Item {
		timespans = append(timespans, item.Metadata.Timespan...)
	}

	var out []MetadataGroupInstance
	for _, ts := range timespans {
		out = collectGroupInstances(ts.Group, "stl_subtitle", ts.Start, ts.End, out)
	}

	assert.Equal(t, []MetadataGroupInstance{
		{UUID: "uuid-1", Start: "-INF", End: "+INF"},
		{UUID: "uuid-3", Start: "-INF", End: "+INF"},
		{UUID: "uuid-4", Start: "0@PAL", End: "250@PAL"},
	}, out)
}

func Test_MetadataDocumentJSON_BareDocument(t *testing.T) {
	bareDoc := `{"timespan":[{"start":"-INF","end":"+INF","group":[{"uuid":"uuid-9","name":"stl_subtitle"}]}]}`

	doc := metadataDocumentJSON{}
	assert.NoError(t, json.Unmarshal([]byte(bareDoc), &doc))

	var out []MetadataGroupInstance
	for _, ts := range doc.Timespan {
		out = collectGroupInstances(ts.Group, "stl_subtitle", ts.Start, ts.End, out)
	}

	assert.Equal(t, []MetadataGroupInstance{{UUID: "uuid-9", Start: "-INF", End: "+INF"}}, out)
}
