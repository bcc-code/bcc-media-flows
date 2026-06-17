package vsapi

import (
	"encoding/xml"
	"strconv"
)

const trashSearchQuery = `<ItemSearchDocument xmlns="http://xml.vidispine.com/schema/vidispine">
    <field>
        <name>portal_deleted</name>
        <range>
			<value>1000-01-01</value>
			<value>3000-01-01</value>
        </range>
    </field>
</ItemSearchDocument>`

type SearchResult struct {
	Hits int `json:"hits"`
	//Suggestion   []interface{} `json:"suggestion"`
	//Autocomplete []interface{} `json:"autocomplete"`
	Entry []Entry `json:"entry"`
}

type Timespan struct {
	Field []interface{} `json:"field"`
	Start string        `json:"start"`
	End   string        `json:"end"`
}

type Entry struct {
	Type     string     `json:"type"`
	ID       string     `json:"id"`
	Item     Item       `json:"item"`
	Timespan []Timespan `json:"timespan"`
	Start    string     `json:"start"`
	End      string     `json:"end"`
}

type itemSearchField struct {
	XMLName xml.Name `xml:"field"`
	Name    string   `xml:"name"`
	Value   string   `xml:"value"`
}

type itemSearchDocument struct {
	XMLName xml.Name          `xml:"ItemSearchDocument"`
	Xmlns   string            `xml:"xmlns,attr"`
	Fields  []itemSearchField `xml:"field"`
}

// SearchByMetadataField returns the IDs of items whose metadata field `name` equals `value`.
// Uses the same /search endpoint as GetTrash.
func (c *Client) SearchByMetadataField(name, value string) ([]string, error) {
	body, err := xml.Marshal(itemSearchDocument{
		Xmlns:  "http://xml.vidispine.com/schema/vidispine",
		Fields: []itemSearchField{{Name: name, Value: value}},
	})
	if err != nil {
		return nil, err
	}

	result := &SearchResult{}
	req := c.restyClient.R()
	req.SetHeader("Content-Type", "application/xml")
	req.SetResult(result)
	req.QueryParam.Add("terse", "true")
	req.SetBody(body)

	if _, err := req.Put(c.baseURL + "/search"); err != nil {
		return nil, err
	}

	ids := make([]string, 0, len(result.Entry))
	for _, entry := range result.Entry {
		ids = append(ids, entry.ID)
	}
	return ids, nil
}

// searchFieldMulti is a single <field> criterion that may carry several
// <value> elements. Multiple values on one field are OR-ed together by
// Vidispine, so it is the right shape for "media type is video OR audio".
type searchFieldMulti struct {
	XMLName xml.Name `xml:"field"`
	Name    string   `xml:"name"`
	Values  []string `xml:"value"`
}

// searchFacetRequest asks Vidispine to return value counts for a field.
type searchFacetRequest struct {
	XMLName xml.Name `xml:"facet"`
	Field   string   `xml:"field"`
}

// fullItemSearchDocument is the search body used by SearchItems. Element order
// (text, field, facet) follows the ItemSearchDocument schema sequence.
type fullItemSearchDocument struct {
	XMLName xml.Name             `xml:"ItemSearchDocument"`
	Xmlns   string               `xml:"xmlns,attr"`
	Text    string               `xml:"text,omitempty"`
	Fields  []searchFieldMulti   `xml:"field,omitempty"`
	Facets  []searchFacetRequest `xml:"facet,omitempty"`
}

// SearchFacetCount is one value/count pair within a facet result.
type SearchFacetCount struct {
	FieldValue string `json:"fieldValue"`
	Count      int    `json:"count"`
}

// SearchFacet is the per-field facet result returned alongside a search.
type SearchFacet struct {
	Field string             `json:"field"`
	Count []SearchFacetCount `json:"count"`
}

// ItemSearchResult is the parsed ItemListDocument returned by SearchItems.
// It reuses MetadataResult (the same terse-metadata shape GetMetadata returns).
type ItemSearchResult struct {
	Hits  int               `json:"hits"`
	Items []*MetadataResult `json:"item"`
	Facet []SearchFacet     `json:"facet"`
}

// ItemSearchParams configures a SearchItems call.
type ItemSearchParams struct {
	// Text is a free-text query. Vidispine matches it against indexed metadata
	// (the item title is indexed), so this drives the "search titles" use case.
	Text string
	// MediaTypes optionally restricts results to these mediaType values
	// (e.g. "video", "audio", "image"). Empty means no media-type filter.
	MediaTypes []string
	// First is the 1-based offset of the first result (Vidispine default 1).
	First int
	// Number is the page size (Vidispine default 100, max 1000).
	Number int
	// Facet, when true, requests mediaType value counts in the response.
	Facet bool
}

// SearchItems runs a paginated item search and returns the items' terse
// metadata plus optional mediaType facet counts. It mirrors the PUT /item
// search documented at https://apidoc.vidispine.com/ (ItemSearchDocument body,
// content=metadata&terse=true for the metadata, first/number for pagination).
func (c *Client) SearchItems(p ItemSearchParams) (*ItemSearchResult, error) {
	if p.First <= 0 {
		p.First = 1
	}
	if p.Number <= 0 {
		p.Number = 100
	}

	doc := fullItemSearchDocument{
		Xmlns: "http://xml.vidispine.com/schema/vidispine",
		Text:  p.Text,
	}
	if len(p.MediaTypes) > 0 {
		doc.Fields = append(doc.Fields, searchFieldMulti{Name: "mediaType", Values: p.MediaTypes})
	}
	if p.Facet {
		doc.Facets = append(doc.Facets, searchFacetRequest{Field: "mediaType"})
	}

	body, err := xml.Marshal(doc)
	if err != nil {
		return nil, err
	}

	result := &ItemSearchResult{}
	req := c.restyClient.R()
	req.SetHeader("Content-Type", "application/xml")
	req.SetResult(result)
	req.QueryParam.Add("content", "metadata")
	req.QueryParam.Add("terse", "true")
	req.QueryParam.Add("first", strconv.Itoa(p.First))
	req.QueryParam.Add("number", strconv.Itoa(p.Number))
	req.SetBody(body)

	if _, err := req.Put(c.baseURL + "/item"); err != nil {
		return nil, err
	}

	return result, nil
}

func (c *Client) GetTrash() ([]string, error) {
	trash := &SearchResult{}

	url := c.baseURL + "/search"
	req := c.restyClient.R()
	req.SetHeader("Content-Type", "application/xml")
	req.SetResult(trash)
	req.QueryParam.Add("terse", "true")
	req.SetBody(trashSearchQuery)

	resp, err := req.Put(url)
	if err != nil {
		return nil, err
	}

	sr := resp.Result().(*SearchResult)

	items := []string{}
	for _, entry := range sr.Entry {
		items = append(items, entry.ID)
	}

	return items, err
}
