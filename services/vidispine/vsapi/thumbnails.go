package vsapi

import (
	"fmt"
	"net/url"
)

// uriListDocument is Vidispine's URIListDocument, a flat list of resource URIs.
type uriListDocument struct {
	URI []string `json:"uri"`
}

// GetThumbnailResources returns the thumbnail-resource URIs registered on an
// item (GET /item/{id}/thumbnailresource). Each URI is API-relative, e.g.
// "/API/thumbnail/VX-45". Frames are then fetched from "{resource}/{time}".
func (c *Client) GetThumbnailResources(itemID string) ([]string, error) {
	resp, err := c.restyClient.R().
		SetResult(&uriListDocument{}).
		Get(c.baseURL + "/item/" + url.PathEscape(itemID) + "/thumbnailresource")
	if err != nil {
		return nil, err
	}
	if resp.IsError() {
		return nil, fmt.Errorf("listing thumbnail resources for %s failed (status %d): %s", itemID, resp.StatusCode(), string(resp.Body()))
	}
	return resp.Result().(*uriListDocument).URI, nil
}

// GetThumbnail fetches a single thumbnail frame for an item as raw image bytes
// and returns the response Content-Type. timeSpec is a Vidispine time string,
// e.g. "0", "250" or "5@PAL"; passing "" returns the resource's default frame.
//
// Auth is the client's configured basic auth, so the bytes must be served by
// the application (the browser cannot fetch these directly).
func (c *Client) GetThumbnail(itemID, timeSpec string) ([]byte, string, error) {
	resources, err := c.GetThumbnailResources(itemID)
	if err != nil {
		return nil, "", err
	}
	if len(resources) == 0 {
		return nil, "", fmt.Errorf("no thumbnail resources for %s", itemID)
	}

	// Resource URIs are API-relative; rebuild an absolute URL against the host
	// of baseURL (which already ends in /API) so resty does not double the path.
	base, err := url.Parse(c.baseURL)
	if err != nil {
		return nil, "", err
	}
	root := base.Scheme + "://" + base.Host

	full := root + resources[0]
	if timeSpec != "" {
		full += "/" + url.PathEscape(timeSpec)
	}

	resp, err := c.restyClient.R().
		SetHeader("Accept", "image/jpeg").
		Get(full)
	if err != nil {
		return nil, "", err
	}
	if resp.IsError() {
		return nil, "", fmt.Errorf("fetching thumbnail for %s failed (status %d)", itemID, resp.StatusCode())
	}

	contentType := resp.Header().Get("Content-Type")
	if contentType == "" {
		contentType = "image/jpeg"
	}
	return resp.Body(), contentType, nil
}
