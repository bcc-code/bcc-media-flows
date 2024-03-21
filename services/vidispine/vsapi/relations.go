package vsapi

// NOTE: Adding relations via Visidpine API seems to somehow mess with Cantemo Portal's ability to display the media asset.
// It is likely that it can be fixed but right now it is not a priority so use Cantemo API to add relations instead.

type RelationResult struct {
	Relations []Relation `json:"relation"`
}
type Direction struct {
	Source string `json:"source"`
	Target string `json:"target"`
	Type   string `json:"type"`
}
type Value struct {
	Value string `json:"value"`
	Key   string `json:"key"`
}
type Relation struct {
	ID        string    `json:"id"`
	Direction Direction `json:"direction"`
	Value     []Value   `json:"value"`
}

// GetRelations returns the relations of a media asset.
func (c *Client) GetRelations(assetID string) ([]Relation, error) {
	relations := &RelationResult{}
	url := c.baseURL + "/item/" + assetID + "/relation"
	resp, err := c.restyClient.R().SetResult(relations).Get(url)
	if err != nil {
		return nil, err
	}

	return resp.Result().(*RelationResult).Relations, nil
}
