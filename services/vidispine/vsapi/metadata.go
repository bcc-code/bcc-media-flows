package vsapi

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"

	"github.com/bcc-code/bcc-media-flows/services/vidispine/vscommon"
)

type MetadataField struct {
	End   string `json:"end"`
	Start string `json:"start"`
	UUID  string `json:"uuid"`
	Value string `json:"value"`
}

type MetadataResult struct {
	Terse map[string]([]*MetadataField) `json:"terse"`
	ID    string                        `json:"id"`
}

// Get returns the first value of the given key, or the fallback if the key is not present
// It does not check what clip the metadata belongs to!
func (m *MetadataResult) Get(key vscommon.FieldType, fallback string) string {
	if val, ok := m.Terse[key.Value]; !ok {
		return fallback
	} else if len(val) == 0 {
		return fallback
	} else {
		return val[0].Value
	}
}

func (m *MetadataResult) GetArray(key vscommon.FieldType) []string {
	if val, ok := m.Terse[key.Value]; !ok {
		return []string{}
	} else {
		out := []string{}
		for _, v := range val {
			out = append(out, v.Value)
		}
		return out
	}
}

func (c *Client) GetMetadata(vsID string) (*MetadataResult, error) {
	url := c.baseURL + "/item/" + vsID + "?content=metadata&terse=true"

	resp, err := c.restyClient.R().
		SetResult(&MetadataResult{}).
		Get(url)

	if err != nil {
		return nil, err
	}

	return resp.Result().(*MetadataResult), nil
}

func (c *Client) GetMetadataFields(vsID string, fields []string) (*MetadataResult, error) {
	reqURL := c.baseURL + "/item/" + vsID + "?content=metadata&terse=true"
	for _, f := range fields {
		reqURL += "&field=" + f
	}

	resp, err := c.restyClient.R().
		SetResult(&MetadataResult{}).
		Get(reqURL)

	if err != nil {
		return nil, err
	}

	return resp.Result().(*MetadataResult), nil
}

type GetMetadataAdvancedParams struct {
	ItemID string
	Group  string
	InTC   float64
	OutTC  float64
}

func (c *Client) GetMetadataAdvanced(params GetMetadataAdvancedParams) (*MetadataResult, error) {
	inString := fmt.Sprintf("%.2f", params.InTC)
	outString := fmt.Sprintf("%.2f", params.OutTC)
	url := fmt.Sprintf("%s/item/%s?content=metadata&terse=true&sampleRate=PAL&interval=%s-%s&group=%s", c.baseURL, params.ItemID, inString, outString, params.Group)

	// Asking for a group/interval with no subclips is the normal case for an item
	// without chapters, and Vidispine may answer that with 404.
	resp, err := tolerating404(c.restyClient.R()).
		SetResult(&MetadataResult{}).
		Get(url)

	if err != nil {
		return nil, err
	}

	return resp.Result().(*MetadataResult), nil
}

// MetadataGroupInstance identifies one occurrence of a named metadata group on an
// item: the group's uuid and the timespan it lives in.
type MetadataGroupInstance struct {
	UUID  string
	Start string
	End   string
}

// The non-terse metadata endpoint answers with a MetadataListDocument
// ({"item":[{"metadata":{"timespan":[...]}}]}); a bare MetadataDocument carries
// the timespans at the top level. metadataDocumentJSON accepts both.
type metadataDocumentJSON struct {
	Item []struct {
		Metadata struct {
			Timespan []metadataTimespanJSON `json:"timespan"`
		} `json:"metadata"`
	} `json:"item"`
	Timespan []metadataTimespanJSON `json:"timespan"`
}

type metadataTimespanJSON struct {
	Start string              `json:"start"`
	End   string              `json:"end"`
	Group []metadataGroupJSON `json:"group"`
}

type metadataGroupJSON struct {
	UUID  string              `json:"uuid"`
	Name  string              `json:"name"`
	Group []metadataGroupJSON `json:"group"`
}

func collectGroupInstances(groups []metadataGroupJSON, name, start, end string, out []MetadataGroupInstance) []MetadataGroupInstance {
	for _, g := range groups {
		if g.Name == name && g.UUID != "" {
			out = append(out, MetadataGroupInstance{UUID: g.UUID, Start: start, End: end})
		}
		out = collectGroupInstances(g.Group, name, start, end, out)
	}
	return out
}

// GetMetadataGroupInstances lists every occurrence of the named metadata group on
// the item, across all timespans (nested groups included).
func (c *Client) GetMetadataGroupInstances(itemID, groupName string) ([]MetadataGroupInstance, error) {
	requestURL, _ := url.Parse(c.baseURL)
	requestURL.Path += fmt.Sprintf("/item/%s/metadata", url.PathEscape(itemID))
	q := requestURL.Query()
	q.Set("group", groupName)
	requestURL.RawQuery = q.Encode()

	// An item with no instances of the group can come back as 404.
	resp, err := tolerating404(c.restyClient.R()).
		SetResult(&metadataDocumentJSON{}).
		Get(requestURL.String())
	if err != nil {
		return nil, err
	}

	doc := resp.Result().(*metadataDocumentJSON)
	timespans := doc.Timespan
	for _, item := range doc.Item {
		timespans = append(timespans, item.Metadata.Timespan...)
	}

	var out []MetadataGroupInstance
	for _, ts := range timespans {
		out = collectGroupInstances(ts.Group, groupName, ts.Start, ts.End, out)
	}
	return out, nil
}

// DeleteMetadataGroupInstances removes every occurrence of the named metadata group
// from the item and returns how many were removed. Removal is addressed by group
// uuid per timespan — addressing by name is what Vidispine rejects as "ambiguous
// path to group" when the name resolves to more than one path.
func (c *Client) DeleteMetadataGroupInstances(itemID, groupName string) (int, error) {
	instances, err := c.GetMetadataGroupInstances(itemID, groupName)
	if err != nil {
		return 0, err
	}
	if len(instances) == 0 {
		return 0, nil
	}

	body, err := createRemoveMetadataGroupsXml(instances)
	if err != nil {
		return 0, err
	}

	requestURL, _ := url.Parse(c.baseURL)
	requestURL.Path += fmt.Sprintf("/item/%s/metadata", url.PathEscape(itemID))

	_, err = c.restyClient.R().
		SetHeader("content-type", "application/xml").
		SetBody(body.String()).
		Put(requestURL.String())
	if err != nil {
		return 0, err
	}

	return len(instances), nil
}

type ItemMetadataFieldParams struct {
	ItemID  string
	GroupID string
	StartTC string
	EndTC   string
	Key     string
	Value   string
}

func (c *Client) SetItemMetadataField(params ItemMetadataFieldParams) error {
	requestURL, _ := url.Parse(c.baseURL)
	requestURL.Path += fmt.Sprintf("/item/%s/metadata", url.PathEscape(params.ItemID))
	q := requestURL.Query()
	requestURL.RawQuery = q.Encode()

	body, err := createSetItemMetadataFieldXml(
		xmlSetItemMetadataFieldParams{
			StartTC: params.StartTC,
			EndTC:   params.EndTC,
			GroupID: params.GroupID,
			Key:     params.Key,
			Value:   params.Value,
			Add:     false,
		},
	)
	if err != nil {
		return err
	}

	_, err = c.restyClient.R().
		SetHeader("content-type", "application/xml").
		SetBody(body.String()).
		Put(requestURL.String())

	if err != nil {
		return err
	}

	return nil
}

func (c *Client) AddToItemMetadataField(params ItemMetadataFieldParams) error {
	requestURL, _ := url.Parse(c.baseURL)
	requestURL.Path += fmt.Sprintf("/item/%s/metadata", url.PathEscape(params.ItemID))
	q := requestURL.Query()
	requestURL.RawQuery = q.Encode()

	body, err := createSetItemMetadataFieldXml(
		xmlSetItemMetadataFieldParams{
			StartTC: params.StartTC,
			EndTC:   params.EndTC,
			GroupID: params.GroupID,
			Key:     params.Key,
			Value:   params.Value,
			Add:     true,
		},
	)
	if err != nil {
		return err
	}

	_, err = c.restyClient.R().
		SetHeader("content-type", "application/xml").
		SetBody(body.String()).
		Put(requestURL.String())

	if err != nil {
		return err
	}

	return nil
}

// GetInOut returns the in and out point of the clip in seconds, suitable
// for use with ffmpeg
func (m *MetadataResult) GetInOut(beginTC string) (float64, float64, error) {
	var v *MetadataField
	if val, ok := m.Terse[vscommon.FieldTitle.Value]; !ok || len(val) == 0 {
		// This should not happen as everything should have a title
		return 0, 0, errors.New("Missing title")
	} else {
		v = val[0]
	}

	start := 0.0
	if v.Start == MinusInf && v.End == PlusInf {
		// This is a full asset so we return 0.0 start and the lenght of the asset as end
		endString := m.Get(vscommon.FieldDurationSeconds, "0")
		end, err := strconv.ParseFloat(endString, 64)
		return start, end, err
	}

	// Now we are in subclip territory. Here we need to extract the TC of the in and out point
	// and convert it to seconds for use with ffmpeg

	inTCseconds, err := vscommon.TCToSeconds(v.Start)
	if err != nil {
		return 0, 0, err
	}

	outTCseconds, err := vscommon.TCToSeconds(v.End)
	if err != nil {
		return 0, 0, err
	}

	// This is basically the offset of the tc that we have to remove from the in and out point
	beginTCseconds, err := vscommon.TCToSeconds(beginTC)
	if err != nil {
		return 0, 0, err
	}

	return inTCseconds - beginTCseconds, outTCseconds - beginTCseconds, nil
}
