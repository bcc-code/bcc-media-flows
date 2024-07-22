package vsapi

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
