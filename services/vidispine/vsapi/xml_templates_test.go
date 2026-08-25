package vsapi

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCreateRemoveMetadataGroupsXml_Empty(t *testing.T) {
	buf, err := createRemoveMetadataGroupsXml(nil)
	assert.NoError(t, err)
	assert.NotContains(t, buf.String(), "<timespan")
}

func TestCreateRemoveMetadataGroupsXml_Single(t *testing.T) {
	buf, err := createRemoveMetadataGroupsXml([]MetadataGroupInstance{
		{UUID: "uuid-1", Start: "-INF", End: "+INF"},
	})
	assert.NoError(t, err)
	out := buf.String()
	assert.Contains(t, out, `<timespan start="-INF" end="+INF">`)
	assert.Contains(t, out, `<group uuid="uuid-1" mode="remove"/>`)
}

func TestCreateRemoveMetadataGroupsXml_Multiple(t *testing.T) {
	buf, err := createRemoveMetadataGroupsXml([]MetadataGroupInstance{
		{UUID: "uuid-1", Start: "0@PAL", End: "250@PAL"},
		{UUID: "uuid-2", Start: "250@PAL", End: "500@PAL"},
	})
	assert.NoError(t, err)
	out := buf.String()
	assert.Contains(t, out, `<group uuid="uuid-1" mode="remove"/>`)
	assert.Contains(t, out, `<group uuid="uuid-2" mode="remove"/>`)
	assert.Contains(t, out, `<timespan start="0@PAL" end="250@PAL">`)
	assert.Contains(t, out, `<timespan start="250@PAL" end="500@PAL">`)
}
