package vsapi

import (
	"encoding/xml"
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
