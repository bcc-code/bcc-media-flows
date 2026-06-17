package vsapi

import (
	"encoding/xml"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_fullItemSearchDocument_XML(t *testing.T) {
	doc := fullItemSearchDocument{
		Xmlns:  "http://xml.vidispine.com/schema/vidispine",
		Text:   "living faith",
		Fields: []searchFieldMulti{{Name: "mediaType", Values: []string{"video", "audio"}}},
		Facets: []searchFacetRequest{{Field: "mediaType"}},
	}

	out, err := xml.Marshal(doc)
	assert.NoError(t, err)

	s := string(out)
	assert.Contains(t, s, `xmlns="http://xml.vidispine.com/schema/vidispine"`)
	assert.Contains(t, s, "<text>living faith</text>")
	assert.Contains(t, s, "<name>mediaType</name>")
	assert.Contains(t, s, "<value>video</value>")
	assert.Contains(t, s, "<value>audio</value>")
	assert.Contains(t, s, "<facet><field>mediaType</field></facet>")

	// text must precede field, which must precede facet (schema sequence).
	assert.Less(t, strings.Index(s, "<text>"), strings.Index(s, "<field>"))
	assert.Less(t, strings.Index(s, "<field>"), strings.Index(s, "<facet>"))
}

func Test_fullItemSearchDocument_XML_TextEscaped(t *testing.T) {
	doc := fullItemSearchDocument{
		Xmlns: "http://xml.vidispine.com/schema/vidispine",
		Text:  "a & b <c>",
	}
	out, err := xml.Marshal(doc)
	assert.NoError(t, err)
	assert.Contains(t, string(out), "a &amp; b &lt;c&gt;")
}

func Test_GetTrash(t *testing.T) {
	if os.Getenv("VIDISPINE_BASE_URL") == "" {
		t.Skip("VIDISPINE_BASE_URL not set")
	}

	if os.Getenv("VIDISPINE_USERNAME") == "" {
		t.Skip("VIDISPINE_USERNAME not set")
	}

	if os.Getenv("VIDISPINE_PASSWORD") == "" {
		t.Skip("VIDISPINE_PASSWORD not set")
	}

	c := NewClient(os.Getenv("VIDISPINE_BASE_URL"), os.Getenv("VIDISPINE_USERNAME"), os.Getenv("VIDISPINE_PASSWORD"))

	res, err := c.GetTrash()
	assert.NoError(t, err)
	assert.NotEmpty(t, res)
}
