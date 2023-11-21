package vsapi

import "fmt"

type CollectionItemsResult struct {
	Hits  int64             `json:"hits"`
	Items []*MetadataResult `json:"item"`
}

// GetItemsInCollection returns the items in a collection
// number is the number of items to return. vidispine default is 100.
// Vidispine max is 1000.
func (c *Client) GetItemsInCollection(collectionVxId string, number int) (*CollectionItemsResult, error) {
	if number > 1000 {
		return nil, fmt.Errorf("number must be less than or equal to 1000. vidispine max is 1000, default is 100, got %d", number)
	}
	url := fmt.Sprintf("%s/collection/%s/item?content=metadata&children=item&number=%d&terse=true", c.baseURL, collectionVxId, number)

	resp, err := c.restyClient.R().
		SetResult(&CollectionItemsResult{}).
		Get(url)

	if err != nil {
		return nil, err
	}

	return resp.Result().(*CollectionItemsResult), nil
}
